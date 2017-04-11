package sodigo;

import "net"
import "log"
import "errors"
import "sync"
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
	sendLock *sync.Mutex
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
		sendLock: &sync.Mutex{},
		resultChans: make(map[uint64]*ResultChan),
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

	log.Println("Handshake Sent")

	// read ready packet
	f, err = packet.ReadFrame(c.conn)
	if err != nil { return nil, err }

	var m proto.Message

	m, err = f.Parse()
	if err != nil { return nil, err }

	pr, _ := m.(*packet.PacketReady)
	if pr == nil  { return nil, errors.New("Bad Initialization Response") }

	log.Println("Connection Ready: client_id =", pr.ClientId, "node_id =", pr.NodeId)

	go c.recvLoop()		// recvLoop, turns net.Conn#Read to chan

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

	log.Println("Will Invoke", seqId, ":", name, method, arguments)

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
	f.Write(c.conn)

	// Wait for invocation
	r := <- resultChan

	if r == nil {
		return "", errors.New("Client Closed")
	}

	log.Println("Did Recive", seqId, ":", r.Result)

	return r.Result, nil
}

func (c *Client) Close() {
	if c.IsConnected {
		// clear isConnected flag
		c.IsConnected = false
		c.conn.Close()
	}
}

// Internal Loop

func (c *Client) recvLoop() {
	defer func() {
		c.Close()
	}()
	for {
		// keep reading frames
		f, err := packet.ReadFrame(c.conn)
		if err == nil {
			go c.handleFrame(f)
		} else {
			// Ignore UnsynchronizedError
			_, ok := err.(packet.UnsynchronizedError)
			if ok { 
				continue 
			} else {
				break
			}
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
		rp.Write(c.conn)
	} else {
		p, ok := m.(*packet.PacketCallerRecv)
		if !ok { return }
		// find cached invocation context
		ch := c.resultChans[p.Id]
		// notify waiting chan
		if ch != nil { *ch <- p }
		// delete invocation context
		delete(c.resultChans, p.Id)
	}
}
