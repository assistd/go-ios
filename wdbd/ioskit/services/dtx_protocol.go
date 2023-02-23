package services

import (
	"bytes"
	"encoding/binary"
	"errors"

	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
)

type DTXMessageHeader struct {
	Magic             uint32
	HeaderLength      uint32
	FragmentId        uint16
	FragmentCount     uint16
	PayloadLength     uint32
	Identifier        uint32
	ConversationIndex uint32
	ChannelCode       uint32
	ExpectsReply      uint32
}

type DTXPayloadHeader struct {
	Flags              uint32
	AuxiliaryLength    uint32
	TotalPayloadLength uint64
}

func (d *DTXMessageHeader) ReadFrom(b []byte) error {
	d.Magic = binary.LittleEndian.Uint32(b)
	d.HeaderLength = binary.LittleEndian.Uint32(b[4:])
	d.FragmentId = binary.LittleEndian.Uint16(b[8:])
	d.FragmentCount = binary.LittleEndian.Uint16(b[10:])
	d.PayloadLength = binary.LittleEndian.Uint32(b[12:])
	d.Identifier = binary.LittleEndian.Uint32(b[16:])
	d.ConversationIndex = binary.LittleEndian.Uint32(b[20:])
	d.ChannelCode = binary.LittleEndian.Uint32(b[24:])
	d.ExpectsReply = binary.LittleEndian.Uint32(b[28:])
	return nil
}

func (d *DTXMessageHeader) Length() int {
	return 0x20
}

func (d *DTXPayloadHeader) ReadFrom(b []byte) error {
	d.Flags = binary.LittleEndian.Uint32(b)
	d.AuxiliaryLength = binary.LittleEndian.Uint32(b[4:])
	d.TotalPayloadLength = binary.LittleEndian.Uint64(b[8:])
	return nil
}

type DTXMessage struct {
	DTXMessageHeader
	DTXPayloadHeader
	Payload []byte
}

type ChannelFragmenter struct {
	FragmentCount     uint16
	Identifier        uint32
	ConversationIndex uint32
	ChannelCode       uint32
	ExpectsReply      uint32

	buf      bytes.Buffer
	finished bool
}

func (c *ChannelFragmenter) AddFirst(header *DTXMessageHeader) {
	c.FragmentCount = header.FragmentCount
	c.Identifier = header.Identifier
	c.ConversationIndex = header.ConversationIndex
	c.ChannelCode = header.ChannelCode
	c.ExpectsReply = header.ExpectsReply
}

func (c *ChannelFragmenter) Add(header *DTXMessageHeader, chunk []byte) {
	c.buf.Write(chunk)
	if header.FragmentId == header.FragmentCount-1 {
		// last fragment
		c.finished = true
	}
}

func (c *ChannelFragmenter) IsFull() bool {
	return c.finished
}

func (c *ChannelFragmenter) Get() ([]byte, error) {
	if !c.finished {
		return nil, errors.New("fragments is not full")
	}

	return c.buf.Bytes(), nil
}

func (c *ChannelFragmenter) Parse() (pheader DTXPayloadHeader, payload []interface{}, auxiliary dtx.PrimitiveDictionary, err error) {
	b := c.buf.Bytes()
	pheader.ReadFrom(b)

	if pheader.AuxiliaryLength > 0 {
		auxiliary = dtx.DecodeAuxiliary(b[16:])
	}

	if pheader.TotalPayloadLength-uint64(pheader.AuxiliaryLength) > 0 {
		pb = b[16+int(pheader.AuxiliaryLength):]
		payload, err := nskeyedarchiver.Unarchive(pb)
	}
}
