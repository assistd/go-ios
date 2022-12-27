package ioskit

import (
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"io"
	"reflect"
)

//IosMuxConn can send and read messages to the usbmuxd
type IosMuxConn struct {
	//tag will be incremented for every message, so responses can be correlated to requests
	tag  uint32
	conn io.ReadWriter
}

func (muxConn *IosMuxConn) ReadMessage() (ios.UsbMuxMessage, error) {
	if muxConn.conn == nil {
		return ios.UsbMuxMessage{}, io.EOF
	}

	msg, err := muxConn.decode()
	if err != nil {
		return ios.UsbMuxMessage{}, err
	}
	return msg, nil
}

// Send sends and encodes a Plist using the usbmux Encoder. Increases the connection tag by one.
func (muxConn *IosMuxConn) Send(msg interface{}) error {
	if muxConn.conn == nil {
		return io.EOF
	}
	muxConn.tag++
	err := muxConn.encode(msg)
	if err != nil {
		log.Error("Error sending mux")
		return err
	}
	return nil
}

//SendMuxMessage serializes and sends a UsbMuxMessage to the underlying DeviceConnection.
//This does not increase the tag on the connection.
func (muxConn *IosMuxConn) SendMuxMessage(msg ios.UsbMuxMessage) error {
	if muxConn.conn == nil {
		return io.EOF
	}

	err := binary.Write(muxConn.conn, binary.LittleEndian, msg.Header)
	if err != nil {
		return err
	}
	_, err = muxConn.conn.Write(msg.Payload)
	return err
}

//encode serializes a MuxMessage struct to a Plist and writes it to the io.Writer.
func (muxConn *IosMuxConn) encode(message interface{}) error {
	log.Debug("UsbMux send", reflect.TypeOf(message), " on ", muxConn.conn)
	mbytes := ios.ToPlistBytes(message)

	// write header
	header := ios.UsbMuxHeader{
		Length:  16 + uint32(len(mbytes)),
		Request: 8,
		Version: 1,
		Tag:     muxConn.tag,
	}
	err := binary.Write(muxConn.conn, binary.LittleEndian, header)
	if err != nil {
		return err
	}

	// write payload
	_, err = muxConn.conn.Write(mbytes)
	return err
}

//decode reads all bytes for the next MuxMessage from r io.Reader and
//returns a UsbMuxMessage
func (muxConn *IosMuxConn) decode() (ios.UsbMuxMessage, error) {
	var muxHeader ios.UsbMuxHeader

	err := binary.Read(muxConn.conn, binary.LittleEndian, &muxHeader)
	if err != nil {
		return ios.UsbMuxMessage{}, err
	}

	payloadBytes := make([]byte, muxHeader.Length-16)
	n, err := io.ReadFull(muxConn.conn, payloadBytes)
	if err != nil {
		return ios.UsbMuxMessage{}, fmt.Errorf("error '%s' while reading usbmux package. "+
			"Only %d bytes received instead of %d", err.Error(), n, muxHeader.Length-16)
	}
	log.Debug("UsbMux Receive on ", &muxConn.conn)

	return ios.UsbMuxMessage{Header: muxHeader, Payload: payloadBytes}, nil
}
