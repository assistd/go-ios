package afc

import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
)

const serviceName = "com.apple.afc"

type Connection struct {
	deviceConn    ios.DeviceConnectionInterface
	packageNumber uint64
}

func New(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, serviceName)
	if err != nil {
		return &Connection{}, err
	}
	return &Connection{deviceConn: deviceConn}, nil
}

func (conn *Connection) sendAfcPacketAndAwaitResponse(packet AfcPacket) (AfcPacket, error) {
	err := Encode(packet, conn.deviceConn.Writer())
	if err != nil {
		return AfcPacket{}, err
	}
	return Decode(conn.deviceConn.Reader())
}

func (conn *Connection) checkOperationStatus(status uint64) bool {
	if status == Afc_operation_status || status == Afc_operation_data || status == Afc_operation_file_close || status == Afc_operation_file_open_result {
		return true
	}
	return false
}

func (conn *Connection) Remove(path string) error {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_remove_path, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return nil
}

func (conn *Connection) MakeDir(path string) error {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_make_dir, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return nil
}
