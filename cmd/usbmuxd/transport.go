package main

import (
	"bytes"
	"context"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
	"io"
	"net"
	"sync"
)

type Transport struct {
	Serial      string
	devConn     *ios.DeviceConnection
	clientConn  net.Conn
	selfLocalId uint32
	connMap     map[uint32]io.ReadWriteCloser
	mutex       sync.Mutex
}

// NewTransport init transport
func NewTransport(devConn *ios.DeviceConnection, clientConn net.Conn, serial string) *Transport {
	return &Transport{
		Serial:     serial,
		devConn:    devConn,
		clientConn: clientConn,
		connMap:    make(map[uint32]io.ReadWriteCloser),
	}
}

// Kick kick off the remote adb server's connection
func (t *Transport) Kick() {
}

// HandleLoop run adb packet reading and writing loop
func (t *Transport) HandleLoop() {
	clientMuxConn := ios.NewUsbMuxConnection(ios.NewDeviceConnectionWithConn(t.clientConn))
	devMuxConn := ios.NewUsbMuxConnection(t.devConn)
	go t.proxyMuxConnection(clientMuxConn, devMuxConn)
	//ctx, cancel := context.WithCancel(context.Background())
}

func (t *Transport) proxyMuxConnection(muxOnUnixSocket, muxToDevice *ios.UsbMuxConnection) {
	for {
		request, err := muxOnUnixSocket.ReadMessage()
		if err != nil {
			muxOnUnixSocket.ReleaseDeviceConnection().Close()
			if err == io.EOF {
				log.Errorf("transport: EOF")
				return
			}
			log.Errorln("transport: failed reading UsbMuxMessage", err)
			return
		}

		var decodedRequest map[string]interface{}
		decoder := plist.NewDecoder(bytes.NewReader(request.Payload))
		err = decoder.Decode(&decodedRequest)
		if err != nil {
			log.Errorln("Failed decoding MuxMessage", request, err)
			return
		}

		messageType := decodedRequest["MessageType"]
		switch messageType {
		case MuxMessageTypeListen:
			t.handleListen(muxOnUnixSocket)
			return
		case MuxMessageTypeConnect:
		case MuxMessageTypeListDevices:
		case MuxMessageTypeListListeners:
		case MuxMessageTypeReadBUID:
		case MuxMessageTypeReadPairRecord:
		case MuxMessageTypeSavePairRecord:
		case MuxMessageTypeDeletePairRecord:
		default:
			log.Fatalf("Unexpected command %s received!", messageType)
		}
	}
}

func (t *Transport) handleListen(muxOnUnixSocket *ios.UsbMuxConnection) {
	cleanup := func() {
		d := muxOnUnixSocket.ReleaseDeviceConnection()
		if d != nil {
			d.Close()
		}
	}

	onAdd := func(ctx context.Context, d Device) {
		if d.Properties.SerialNumber != t.Serial {
			return
		}

		d.MessageType = ListenMessageAttached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			log.Errorln("transport: LISTEN: write failed:", err)
			cleanup()
		}
	}

	onRemove := func(ctx context.Context, d Device) {
		d.MessageType = ListenMessageDetached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			log.Errorln("transport: LISTEN: write failed:", err)
			cleanup()
		}
	}
	// TODO: support ListenMessagePaired

	for _, d := range registry.Devices() {
		onAdd(nil, d)
	}

	unListen := registry.Listen(NewDeviceListener(onAdd, onRemove))
	defer unListen()
	defer cleanup()

	//use this to detect when the conn is closed. There shouldn't be any messages received ever.
	_, err := muxOnUnixSocket.ReadMessage()
	log.Error("transport: LISTEN: error on read", err)
}
