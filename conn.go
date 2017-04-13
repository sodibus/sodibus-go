package sodigo

import "net"
import "log"
import "time"
import "errors"
import "github.com/sodibus/packet"

// Delegate for Conn
type ConnDelegate interface {
	ConnPrepareHandshake(c *Conn) *packet.PacketHandshake
	ConnDidReceiveReady(c *Conn, p *packet.PacketReady)
	ConnDidReceiveFrame(c *Conn, f *packet.Frame)
}

// Conn is a resumable net.TCPConn wrapper
//
// Conn use delegate mechanism
type Conn struct {
	addr string
	currConn *net.TCPConn
	sendChan chan *packet.Frame
	delegate ConnDelegate
	isClosed bool
}

func NewConn(addr string, delegate ConnDelegate) *Conn {
	return &Conn{
		sendChan: make(chan *packet.Frame, 16),
		addr: addr,
		delegate: delegate,
	}
}

func (c *Conn) Send(f *packet.Frame) {
	c.sendChan <- f
}

func (c *Conn) Close() {
	// mark as closed
	c.isClosed = true
	// close underlaying if needed
	cn := c.currConn
	if cn != nil { cn.Close() }
}

func (c *Conn) Run() {
	for {
		// run a single connection
		err := c.runSingle()
		log.Println("Conn disconnected", err)

		// if this client is closed quit
		if c.isClosed { break }

		// sleep 3 second and loop
		log.Println("Reconnect after 3s", err)
		time.Sleep(3 * time.Second)
	}
}

// Run a single net.TCPConn
func (c *Conn) runSingle() error {
	// resolve
	tcpAddr, err := net.ResolveTCPAddr("tcp", c.addr)
	if err != nil { return err }

	// connect
	cn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil { return err }

	// set c.currConn
	c.currConn = cn

	// defer to close
	defer func() {
		// unset c.currConn
		c.currConn = nil
		// close the underlaying connection
		cn.Close()
	}()

	// handshake
	p := c.delegate.ConnPrepareHandshake(c)
	f, err := packet.NewFrameWithPacket(p)

	err = f.Write(cn)
	if err != nil { return err }

	f, err = packet.ReadFrame(cn)
	if err != nil { return err }

	m, err := f.Parse()
	if err != nil { return err }

	r, ok := m.(*packet.PacketReady)
	if !ok { return errors.New("Failed to Handshake") }

	// delegate out ready packet
	c.delegate.ConnDidReceiveReady(c, r)

	// make the close/done chan
	close := make(chan bool, 1)
	done 	:= make(chan bool, 1)

	// defer to wait sendLoop done
	defer func() {
		// send close signal
		close <- true
		// wait sendLoop done
		<- done
	} ()

	// send loop
	go c.sendLoop(cn, close, done)

	// recv loop
	for {
		var f *packet.Frame
		f, err = packet.ReadFrame(cn)

		if err != nil {
			_, ok := err.(packet.UnsynchronizedError)
			if !ok { break }
		} else {
			c.delegate.ConnDidReceiveFrame(c, f)
		}
	}

	return err
}

// send loop for a single net.TCPConn
//
// accept a close signal, send to done when finished
func (c *Conn) sendLoop(cn *net.TCPConn, close chan bool, done chan bool) {
	OUTER:
	for {
		select {
			// handle frames
			case f := <- c.sendChan: {
				// write frame
				err := f.Write(cn)
				// if failed to write
				if err != nil {
					// close connection
					cn.Close()
					// resend frame
					go c.Send(f)
					// break the for-loop
					break OUTER
				}
			}
			// handle close signal
			case <- close: {
				// break the for-loop
				break OUTER
			}
		}
	}

	// notify done
	done <- true
}