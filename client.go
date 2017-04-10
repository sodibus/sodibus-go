package sodigo;

import "net"
import "fmt"
import "errors"
import "sync/atomic"
import "encoding/binary"

import "github.com/sodibus/sodigo/packet"
import "github.com/golang/protobuf/proto"

// PacketCallerRecv channel

type InvocationResultChan chan *packet.PacketCallerRecv

// Client structure

type Client struct {
	// sequence id, client-local unique for each Invocation
	seqId uint64
	// client mode, either be Caller or Callee
	mode packet.ClientMode
	// callee names
	provides []string
	// conn
	conn *net.TCPConn
	// channels
	recvChan chan *Frame
	sendChan chan *Frame
	stopChan chan chan bool
	// pending invocations
	invocations map[uint64]*InvocationResultChan
	// state
	isConnected bool
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
		mode: mode,
		provides: provides,
		recvChan: make(chan *Frame, 128),
		sendChan: make(chan *Frame, 128),
		stopChan: make(chan chan bool),
	}

	// dial TCP
	c.conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil { return nil, err }

	// write init packet
	err = c.writePacket(0x01, &packet.PacketInitialization{
		Mode: c.mode,
		Provides: c.provides,
	})
	if err != nil { return nil, err }

	// read ready packet
	var f *Frame
	f, err = c.readFrame()

	if f.Type != 0x02 {
		return nil, errors.New("Bad Initialization Response")
	}

	// parse ready packet
	pr := packet.PacketReady{}
	err = proto.Unmarshal(f.Data, &pr)
	if err != nil { return nil, err }

	fmt.Printf("Connection Ready: As %p of %p", pr.ClientId, pr.NodeId)

	go c.recvLoop()		// recvLoop, turns net.Conn#Read to chan
	go c.handleLoop()	// handleLoop

	c.isConnected = true

	return c, nil
}

// Public Method

func (c *Client) Invoke(name string, arguments []string) (string, error) {
	// Generate a new Id
	seqId := atomic.AddUint64(&c.seqId, 1)

	// Create a waitChan
	waitChan := make(InvocationResultChan)

	// Create a Context and save to invocations
	c.invocations[seqId] = &waitChan

	// Send
	ps := packet.PacketCallerSend{
		Id: seqId,
		Invocation: &packet.Invocation{
			CalleeName: name,
			Arguments: arguments,
			NoReturn: false,
		},
	}

	f, err := NewFrameWithPacket(0x03, &ps)
	if err != nil { return "", err }

	c.sendChan <- f
	// Wait for invocation
	p := <- waitChan

	return p.Result, nil
}

func (c *Client) Close() {
	if c.isConnected {
		c.isConnected = false

		// send stop signal and wait done
		ch := make(chan bool)
		c.stopChan <- ch
		<- ch
	}
}

// Internal Loop

func (c *Client) recvLoop() {
	for {
		f, err := c.readFrame()
		if err == nil {
			c.recvChan <- f
		} else {
			sdError, ok := err.(SdError)
			// ignore Frame Unsynchronized Error
			if ok {
				if sdError.Code == ERR_FrameUnsynchronized {
					continue
				}
			} 
			// close client and end loop
			c.Close()
			break
		}
	}
}

func (c *Client) handleLoop() {
	select {
		// if Frame received, handle it
		case f := <- c.recvChan: {
			go c.handleFrame(f)
		}
		// if new frame to send, send it
		case f := <- c.sendChan: {
			go f.Write(c.conn)
		}
		// if stop signal received, close the connection and end loop
		case ch := <- c.stopChan: {
			c.conn.Close()
			ch <- true
			break
		}
	}
}

func (c *Client) handleFrame(f *Frame) {
	if f.Type == 0x04 {
		p := packet.PacketCallerRecv{}
		proto.Unmarshal(f.Data, &p)

		if p.Id > 0 {
			ch := c.invocations[p.Id]
			if ch != nil {
				*ch <- &p
			}
			delete(c.invocations, p.Id)
		}
	}
}

// I/O

func (c *Client) writePacket(t uint8, m proto.Message) error {
	f, err := NewFrameWithPacket(t, m)
	if err != nil { return err }
	return f.Write(c.conn)
}

func (c *Client) readFrame() (*Frame, error) {
	var err error
	var head = make([]uint8, 1)
	var dataLen = make([]uint8, 4)

	// seek head (1 byte)
	_, err = c.conn.Read(head)
	if err != nil { return nil, err }
	if head[0] != 0xAA {
		return nil, NewSdError(ERR_FrameUnsynchronized)
	}

	// read packet type (1 byte)
	_, err = c.conn.Read(head)
	if err != nil { return nil, err }

	// read dataLen (4 byte, uint32)
	_, err = c.conn.Read(dataLen)
	if err != nil { return nil, err }
	len := binary.BigEndian.Uint32(dataLen)

	// read data
	var data = make([]byte, len)
	_, err = c.conn.Read(data)
	if err != nil { return nil, err }

	// build frame
	return &Frame{ Type: head[0], Data: data }, err
}
