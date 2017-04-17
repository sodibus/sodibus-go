package sodigo

import "log"
import "sync"
import "time"
import "errors"
import "sync/atomic"
import "github.com/sodibus/packet"

// ResultChan chan for PacketCallerRecv
type ResultChan chan *packet.PacketCallerRecv

// CallerClient represents caller client
type CallerClient struct {
	seqID           uint64
	resultChans     map[uint64]ResultChan
	resultChansLock *sync.RWMutex
	conn            *Conn
}

// NewCaller create a new caller
func NewCaller(addr string) *CallerClient {
	cl := &CallerClient{
		resultChans:     make(map[uint64]ResultChan),
		resultChansLock: &sync.RWMutex{},
	}
	cl.conn = NewConn(addr, cl)
	go cl.conn.Run()
	return cl
}

// Invoke invoke a service
func (cl *CallerClient) Invoke(calleeName string, methodName string, arguments []string) (string, error) {
	id := atomic.AddUint64(&cl.seqID, 1)

	ch := make(ResultChan, 1)

	// put resultChan
	cl.resultChansLock.Lock()
	cl.resultChans[id] = ch
	cl.resultChansLock.Unlock()

	defer func() {
		// del resultChan
		cl.resultChansLock.Lock()
		delete(cl.resultChans, id)
		cl.resultChansLock.Unlock()
	}()

	// send packet
	f, err := packet.NewFrameWithPacket(&packet.PacketCallerSend{
		Id: id,
		Invocation: &packet.Invocation{
			CalleeName: calleeName,
			MethodName: methodName,
			Arguments:  arguments,
			NoReturn:   false,
		},
	})
	if err != nil {
		return "", err
	}

	cl.conn.Send(f)

	// wait resultChan
	var r *packet.PacketCallerRecv

	select {
	case r = <-ch:
		{
		}
	case <-time.After(10 * time.Second):
		{
		}
	}

	if r == nil {
		return "", errors.New("Invocation Timeout")
	}
	return r.Result, nil
}

// ConnPrepareHandshake implements delegate logic
func (cl *CallerClient) ConnPrepareHandshake(c *Conn) *packet.PacketHandshake {
	return &packet.PacketHandshake{
		Mode: packet.ClientMode_CALLER,
	}
}

// ConnDidReceiveReady implements delegate logic
func (cl *CallerClient) ConnDidReceiveReady(c *Conn, p *packet.PacketReady) {
	log.Println("Caller Ready node =", p.NodeId, ", client_id=", p.ClientId)
}

// ConnDidReceiveFrame implements delegate logic
func (cl *CallerClient) ConnDidReceiveFrame(c *Conn, f *packet.Frame) {
	m, err := f.Parse()
	if err != nil {
		return
	}

	r, ok := m.(*packet.PacketCallerRecv)
	if !ok {
		return
	}

	cl.resultChansLock.RLock()
	defer cl.resultChansLock.RUnlock()

	ch := cl.resultChans[r.Id]
	if ch != nil {
		ch <- r
	}
}
