package services

import (
	"bytes"
	"encoding/binary"
	"errors"

	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	log "github.com/sirupsen/logrus"
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
	return 32
}

func (d *DTXPayloadHeader) ReadFrom(b []byte) error {
	d.Flags = binary.LittleEndian.Uint32(b)
	d.AuxiliaryLength = binary.LittleEndian.Uint32(b[4:])
	d.TotalPayloadLength = binary.LittleEndian.Uint64(b[8:])
	return nil
}

func (d *DTXPayloadHeader) Length() int {
	return 16
}

type DTXMessage struct {
	DTXMessageHeader
	DTXPayloadHeader
	Payload []byte
}

// The AuxiliaryHeader can actually be completely ignored. We do not need to care about the buffer size
// And we already know the AuxiliarySize. The other two ints seem to be always 0 anyway. Could
// also be that Buffer and Aux Size are Uint64
type DTXAuxiliaryHeader struct {
	BufferSize    uint32
	Unknown       uint32
	AuxiliarySize uint32
	Unknown2      uint32
}

func (d *DTXAuxiliaryHeader) Length() int {
	return 16
}

func (d *DTXAuxiliaryHeader) ReadFrom(b []byte) {
	d.BufferSize = binary.LittleEndian.Uint32(b)
	d.Unknown = binary.LittleEndian.Uint32(b[4:])
	d.AuxiliarySize = binary.LittleEndian.Uint32(b[8:])
	d.Unknown2 = binary.LittleEndian.Uint32(b[12:])
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

	pblen := pheader.Length()
	if pheader.AuxiliaryLength > 0 {
		auxiliary = dtx.DecodeAuxiliary(b[pblen:])
	}

	if pheader.TotalPayloadLength-uint64(pheader.AuxiliaryLength) > 0 {
		pb := b[pblen+int(pheader.AuxiliaryLength):]
		payload, err = nskeyedarchiver.Unarchive(pb)
	}

	return
}

func (c *ChannelFragmenter) Parse2() (payload []interface{}, aux map[string]interface{}, err error) {
	b := c.buf.Bytes()
	pheader := DTXPayloadHeader{}
	pheader.ReadFrom(b)
	pblen := pheader.Length()

	if pheader.AuxiliaryLength > 0 {
		auxheader := DTXAuxiliaryHeader{}
		auxheader.ReadFrom(b[pblen:])
		log.Infof("aux header:%#v", auxheader)

		off := pblen + auxheader.Length()
		auxiliary := dtx.DecodeAuxiliary(b[off : off+int(auxheader.AuxiliarySize)])
		args := auxiliary.GetArguments()
		if len(args) == 0 {
			err = errors.New("empty auxiliary dictionary")
			return
		}

		data, ok := args[0].([]byte)
		if !ok {
			err = errors.New("invalid aux")
			return
		}

		unarchived, e := nskeyedarchiver.Unarchive(data)
		if e != nil {
			err = e
			return
		}

		if len(unarchived) == 0 {
			err = errors.New("unarchived failed")
			return
		}

		a, ok := unarchived[0].(map[string]interface{})
		if !ok {
			err = errors.New("invalid map")
			return
		}

		aux = a
		// log.Infof("aux:%#v", aux)
	}

	if pheader.TotalPayloadLength-uint64(pheader.AuxiliaryLength) > 0 {
		pb := b[pblen+int(pheader.AuxiliaryLength):]
		payload, err = nskeyedarchiver.Unarchive(pb)
		if err != nil {
			return
		}
		if len(payload) != 1 {
			err = errors.New("payload size != 1")
			return
		}

		// log.Infof("payload:%#v", payload[0])
	}

	return
}
