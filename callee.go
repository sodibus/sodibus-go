package sodigo

import "log"
import "github.com/sodibus/packet"

// CalleeHandler handler function for Callee
type CalleeHandler func(calleeName string, methodName string, arguments []string) string

// CalleeClient represents callee client, provides services
type CalleeClient struct {
	conn        *Conn
	calleeNames []string
	Handler     CalleeHandler
}

// NewCallee creates a new Callee
func NewCallee(addr string, calleeNames []string) *CalleeClient {
	cl := &CalleeClient{
		calleeNames: calleeNames,
	}
	cl.conn = NewConn(addr, cl)
	go cl.conn.Run()
	return cl
}

// ConnPrepareHandshake implements delegate logic
func (cl *CalleeClient) ConnPrepareHandshake(c *Conn) *packet.PacketHandshake {
	return &packet.PacketHandshake{
		Mode:     packet.ClientMode_CALLEE,
		Provides: cl.calleeNames,
	}
}

// ConnDidReceiveReady implements delegate logic
func (cl *CalleeClient) ConnDidReceiveReady(c *Conn, p *packet.PacketReady) {
	log.Println("Callee Ready node =", p.NodeId, ", client_id =", p.ClientId, ", provides =", cl.calleeNames)
}

// ConnDidReceiveFrame implements delegate logic
func (cl *CalleeClient) ConnDidReceiveFrame(c *Conn, f *packet.Frame) {
	// parse packet
	m, err := f.Parse()
	if err != nil {
		return
	}

	r, ok := m.(*packet.PacketCalleeRecv)
	if !ok {
		return
	}

	// solve with handler
	rs := cl.Handler(r.Invocation.CalleeName, r.Invocation.MethodName, r.Invocation.Arguments)

	// send result packet
	rf, err := packet.NewFrameWithPacket(&packet.PacketCalleeSend{
		Id:     r.Id,
		Result: rs,
	})
	if err != nil {
		return
	}

	c.Send(rf)
}
