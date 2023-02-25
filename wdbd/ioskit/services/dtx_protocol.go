package services

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

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

type fHeader struct {
	FragmentCount     uint16
	Identifier        uint32
	ConversationIndex uint32
	ChannelCode       uint32
	ExpectsReply      uint32
}

type Fragment struct {
	fHeader
	buf      bytes.Buffer
	finished bool
}
type ChannelFragmenter struct {
	queue   []Fragment
	current Fragment
}

func (c *Fragment) AddFirst(header *DTXMessageHeader) {
	c.FragmentCount = header.FragmentCount
	c.Identifier = header.Identifier
	c.ConversationIndex = header.ConversationIndex
	c.ChannelCode = header.ChannelCode
	c.ExpectsReply = header.ExpectsReply
}

func (c *Fragment) Add(header *DTXMessageHeader, chunk []byte) {
	if c.finished {
		log.Panicf("add to fulled fheader:%#v header:%#v", c.fHeader, header)
	}

	c.buf.Write(chunk)
	if header.FragmentId == header.FragmentCount-1 {
		// last fragment
		c.finished = true
	}
}

func (c *Fragment) IsFull() bool {
	return c.finished
}

func (c *Fragment) reset() {
	c.finished = false
	c.buf.Reset()
}

func (c *Fragment) Get() ([]byte, error) {
	if !c.finished {
		return nil, errors.New("fragments is not full")
	}

	return c.buf.Bytes(), nil
}

func (c *Fragment) Parse() (payload []interface{}, aux map[string]interface{}, err error) {
	_, p, args, e := c.ParseEx()
	payload = p
	if len(args) > 0 {
		_aux, ok := args[0].(map[string]interface{})
		if ok {
			aux = _aux
		}
	}
	err = e
	return
}

func (c *Fragment) ParseEx() (pheader DTXPayloadHeader, payload []interface{}, aux []interface{}, err error) {
	b := c.buf.Bytes()
	pheader.ReadFrom(b)
	pblen := pheader.Length()
	// log.Infof("aux header:%#v", auxheader)

	// payload
	if pheader.TotalPayloadLength-uint64(pheader.AuxiliaryLength) > 0 {
		pb := b[pblen+int(pheader.AuxiliaryLength):]
		payload, err = nskeyedarchiver.Unarchive(pb)
		if err != nil {
			err = fmt.Errorf("unarchived failed:%v", err)
			// log.Errorf("payload:%#v", payload[0])
		}

	}

	// auxiliary
	if pheader.AuxiliaryLength > 0 {
		auxheader := DTXAuxiliaryHeader{}
		auxheader.ReadFrom(b[pblen:])
		off := pblen + auxheader.Length()
		auxiliary := dtx.DecodeAuxiliary(b[off : off+int(auxheader.AuxiliarySize)])
		args := auxiliary.GetArguments()
		// log.Infof("   %v", auxiliary.String())
		aux = make([]interface{}, len(args))
		for i := 0; i < len(args); i++ {
			data, ok := args[i].([]byte)
			if ok {
				if v, e := nskeyedarchiver.Unarchive(data); e == nil {
					aux[i] = v[0]
					continue
				}
			}
			aux[i] = args[i]
		}
	}
	return
}

func (c *ChannelFragmenter) AddFirst(header *DTXMessageHeader) {
	c.current.AddFirst(header)
}

func (c *ChannelFragmenter) Add(header *DTXMessageHeader, chunk []byte) (f Fragment, b bool) {
	c.current.Add(header, chunk)
	if c.current.IsFull() {
		f = c.current
		b = true
		c.queue = append(c.queue, c.current)
		c.current.reset()
		return
	}
	return
}

func (c *ChannelFragmenter) Get() (Fragment, error) {
	if len(c.queue) > 0 {
		f := c.queue[0]
		c.queue = c.queue[1:]
		return f, nil
	}

	return Fragment{}, errors.New("no valid fragment")
}

// All the known MessageTypes
const (
	//Ack is the messagetype for a 16 byte long acknowleding DtxMessage.
	Ack = 0x0
	//Uknown
	UnknownTypeOne = 0x1
	//Methodinvocation is the messagetype for a remote procedure call style DtxMessage.
	Methodinvocation = 0x2
	//ResponseWithReturnValueInPayload is the response for a method call that has a return value
	ResponseWithReturnValueInPayload = 0x3
	//DtxTypeError is the messagetype for a DtxMessage containing an error
	DtxTypeError = 0x4
)

// This is only used for creating nice String() output
var messageTypeLookup = map[int]string{
	Ack:                              `Ack`,
	Methodinvocation:                 `Methodinvocation`,
	ResponseWithReturnValueInPayload: `ResponseWithReturnValueInPayload`,
	DtxTypeError:                     `Error`,
}

func LogDtx(d DTXMessageHeader, p DTXPayloadHeader) string {
	var e = ""
	if d.ExpectsReply == 1 {
		e = "e"
	}

	desc, ok := messageTypeLookup[int(p.Flags)]
	if !ok {
		desc = "Unknown"
	}

	return fmt.Sprintf("i%d.%d%s c%d t:%v[%s] mlen:%d aux_len%d payload%d",
		d.Identifier, d.ConversationIndex, e, d.ChannelCode, p.Flags, desc,
		d.PayloadLength, p.AuxiliaryLength, p.TotalPayloadLength-uint64(p.AuxiliaryLength))
}
