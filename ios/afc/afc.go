package afc

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Afc_magic                      uint64 = 0x4141504c36414643
	Afc_header_size                uint64 = 40
	Afc_fopen_wronly               uint64 = 0x3
	Afc_operation_status           uint64 = 0x1
	Afc_operation_read_dir         uint64 = 0x3
	Afc_operation_file_open        uint64 = 0x0000000D
	Afc_operation_file_close       uint64 = 0x00000014
	Afc_operation_file_write       uint64 = 0x00000010
	Afc_operation_file_open_result uint64 = 0x0000000E
)

type AfcPacketHeader struct {
	Magic         uint64
	Entire_length uint64
	This_length   uint64
	Packet_num    uint64
	Operation     uint64
}

type AfcPacket struct {
	Header        AfcPacketHeader
	HeaderPayload []byte
	Payload       []byte
}

func Decode(reader io.Reader) (AfcPacket, error) {
	var header AfcPacketHeader
	err := binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		return AfcPacket{}, err
	}
	if header.Magic != Afc_magic {
		return AfcPacket{}, fmt.Errorf("Wrong magic:%x expected: %x", header.Magic, Afc_magic)
	}
	headerPayloadLength := header.This_length - Afc_header_size
	headerPayload := make([]byte, headerPayloadLength)
	_, err = io.ReadFull(reader, headerPayload)
	if err != nil {
		return AfcPacket{}, err
	}

	contentPayloadLength := header.Entire_length - header.This_length
	payload := make([]byte, contentPayloadLength)
	_, err = io.ReadFull(reader, payload)
	if err != nil {
		return AfcPacket{}, err
	}
	return AfcPacket{header, headerPayload, payload}, nil
}

func Encode(packet AfcPacket, writer io.Writer) error {
	err := binary.Write(writer, binary.LittleEndian, packet.Header)
	if err != nil {
		return err
	}
	_, err = writer.Write(packet.HeaderPayload)
	if err != nil {
		return err
	}

	_, err = writer.Write(packet.Payload)
	if err != nil {
		return err
	}
	return nil
}
