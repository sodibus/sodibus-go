package sodigo;

import "net"
import "fmt"
import "errors"
import "sync/atomic"

import "github.com/sodibus/packet"
import "github.com/golang/protobuf/proto"

// PacketCallerRecv channel

type ResultChan chan *packet.PacketCallerRecv

type CalleeHandler func (service string, method string, arguments []string) string

// Client structure

type Client struct {
	// sequence id, client-local unique for each Invocation
	SeqId uint64
	// client mode, either be Caller or Callee
	IsCallee bool
	// callee names
	Provides []string
	// conn
	conn *net.TCPConn
	// channels
	recvChan chan *packet.Frame
	sendChan chan *packet.Frame
	stopChan chan chan bool
	// pending resultChans
	resultChans map[uint64]*ResultChan
	// state
	IsConnected bool
	// callee
	Handler CalleeHandler
}

// Create a Client

// create a Caller client
func DialAsCaller(addr string) (*Client, error) {
	return Dial(addr, packet.ClientMode_CALLER, []string{})
}

// create a Callee client
func DialAsCallee(addr string, provides []string) (*Client, error) {
	return Dial(addr, packet.ClientMode_CALLEE, provides)
}

// create a client
func Dial(addr string, mode packet.ClientMode, provides []string) (*Client, error) {
	// resolve TCP address
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil { return nil, err }

	// initialize object
	c := &Client{
		IsCallee: mode == packet.ClientMode_CALLEE,
		Provides: provides,
		recvChan: make(chan *packet.Frame, 128),
		sendChan: make(chan *packet.Frame, 128),
		stopChan: make(chan chan bool),
	}

	// dial TCP
	c.conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil { return nil, err }

	// write init packet
	var f *packet.Frame
	f, err = packet.NewFrameWithPacket(&packet.PacketHandshake{
		Mode: mode,
		Provides: c.Provides,
	})
	if err != nil { return nil, err }

	err = f.Write(c.conn)
	if err != nil { return nil, err }

	// read ready packet
	f, err = packet.ReadFrame(c.conn)

	var m proto.Message

	m, err = f.Parse()
	if err != nil { return nil, err }

	pr, _ := m.(*packet.PacketReady)
	if pr == nil  { return nil, errors.New("Bad Initialization Response") }

	fmt.Printf("Connection Ready: As %p of %p", pr.ClientId, pr.NodeId)

	go c.recvLoop()		// recvLoop, turns net.Conn#Read to chan
	go c.handleLoop()	// handleLoop

	c.IsConnected = true

	return c, nil
}

// Public Method

func (c *Client) Invoke(name string, method string, arguments []string) (string, error) {
	// return if this is a Callee
	if (c.IsCallee) {
		return "", errors.New("Client mode is Callee")
	}

	// return if closed
	if (!c.IsConnected) {
		return "", errors.New("Client Closed")
	}

	// Generate a new Id
	seqId := atomic.AddUint64(&c.SeqId, 1)

	// Create a waitChan
	resultChan := make(ResultChan)

	// Create a Context and save to resultChans
	c.resultChans[seqId] = &resultChan

	// Build frame
	f, err := packet.NewFrameWithPacket(&packet.PacketCallerSend{
		Id: seqId,
		Invocation: &packet.Invocation{
			CalleeName: name,
			MethodName: method,
			Arguments: arguments,
			NoReturn: false,
		},
	})
	if err != nil { return "", err }

	// Send Chan
	c.sendChan <- f

	// Wait for invocation
	r := <- resultChan

	if r == nil {
		return "", errors.New("Client Closed")
	}

	return r.Result, nil
}

func (c *Client) Close() {
	if c.IsConnected {
		// clear isConnected flag
		c.IsConnected = false

		// send stop signal and wait done
		ch := make(chan bool)
		c.stopChan <- ch
		<- ch
	}
}

// Internal Loop

func (c *Client) recvLoop() {
	for {
		// keep reading frames
		f, err := packet.ReadFrame(c.conn)
		if err == nil {
			c.recvChan <- f
		} else {
			// Ignore UnsynchronizedError
			_, ok := err.(packet.UnsynchronizedError)
			if ok { continue } 
			// otherwise close client and end loop
			c.Close()
			break
		}
	}
}

func (c *Client) handleLoop() {
	select {
		// if packet.Frame received, handle it
		case f := <- c.recvChan: {
			go c.handleFrame(f)
		}
		// if new frame to send, send it
		case f := <- c.sendChan: {
			f.Write(c.conn)
		}
		// if stop signal received, close the connection and end loop
		case ch := <- c.stopChan: {
			c.conn.Close()
			ch <- true
			break
		}
	}
}

func (c *Client) handleFrame(f *packet.Frame) {
	// Parse Packet
	m, err := f.Parse()
	if err != nil { return }

	// Try handle CallerRecv
	if c.IsCallee {
		p, ok := m.(*packet.PacketCalleeRecv)
		if !ok { return }
		if c.Handler == nil { return }
		// invoke handler
		r := c.Handler(p.Invocation.CalleeName, p.Invocation.MethodName, p.Invocation.Arguments)
		// build response
		rp, err := packet.NewFrameWithPacket(&packet.PacketCalleeSend{
			Id: p.Id,
			Result: r,
		})
		if err != nil { return }
		// send response
		c.sendChan <- rp
	} else {
		p, ok := m.(*packet.PacketCallerRecv)
		if !ok { return }
		// find cached invocation context
		ch := c.resultChans[p.Id]
		// notify waiting chan
		if ch != nil {
			*ch <- p
		}
		// delete invocation context
		delete(c.resultChans, p.Id)
	}
}
