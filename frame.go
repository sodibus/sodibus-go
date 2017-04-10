package sodigo

import "io"
import "encoding/binary"
import "github.com/golang/protobuf/proto"

type Frame struct {
	Type uint8
	Data []uint8
}

func NewFrameWithPacket(t uint8, m proto.Message) (*Frame, error) {
	data, err := proto.Marshal(m)
	if err != nil { return nil, err }
	return &Frame{ Type: t, Data: data }, nil
}

func (f *Frame) Write(w io.Writer) error {
	var err error
	_, err = w.Write([]uint8{ 0xAA, f.Type })
	if err != nil { return err }
	dataLen := make([]byte, 4)
	binary.BigEndian.PutUint32(dataLen, uint32(len(f.Data)))
	w.Write(dataLen)
	if err != nil { return err }
	w.Write(f.Data)
	if err != nil { return err }
	return err
}
