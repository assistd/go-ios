package main

import (
	"bytes"
	"context"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
	"io"
	"net"
	"os"
	"runtime/debug"
)

const (
	MuxMessageTypeListen           = "Listen"
	MuxMessageTypeConnect          = "Connect"
	MuxMessageTypeListDevices      = "ListDevices"
	MuxMessageTypeListListeners    = "ListListeners"
	MuxMessageTypeReadBUID         = "ReadBUID"
	MuxMessageTypeReadPairRecord   = "ReadPairRecord"
	MuxMessageTypeSavePairRecord   = "SavePairRecord"
	MuxMessageTypeDeletePairRecord = "DeletePairRecord"
)

const (
	ResultOK          = 0
	ResultBadCommand  = 1
	ResultBadDev      = 2
	ResultConnRefused = 3
	ResultBadVersion  = 6
)

const (
	MessageResult       = 1
	MessageConnect      = 2
	MessageListen       = 3
	MessageDeviceAdd    = 4
	MessageDeviceRemove = 5
	MessageDevicePaired = 6
	MessagePlist        = 8
)

const (
	ListenMessageAttached = "Attached"
	ListenMessageDetached = "Detached"
	ListenMessagePaired   = "Paired"
)

const serial = "hello,world"

var deviceId int

func handleListen(muxOnUnixSocket *ios.UsbMuxConnection, muxToDevice *ios.UsbMuxConnection) {
	onAdd := func(ctx context.Context, d Device) {
	}

	onRemove := func(ctx context.Context, d Device) {
	}

	for _, d := range registry.Devices() {
		onAdd(nil, d)
	}

	unListen := registry.Listen(NewDeviceListener(onAdd, onRemove))
	defer unListen()

	go func() {
		//use this to detect when the conn is closed. There shouldn't be any messages received ever.
		_, err := muxOnUnixSocket.ReadMessage()
		if err == io.EOF {
			muxOnUnixSocket.ReleaseDeviceConnection().Close()
			muxToDevice.ReleaseDeviceConnection().Close()
			return
		}
		log.WithFields(log.Fields{"error": err}).Error("Unexpected error on read for LISTEN connection")
	}()

	cleanup := func() {
		d := muxOnUnixSocket.ReleaseDeviceConnection()
		d1 := muxToDevice.ReleaseDeviceConnection()
		if d != nil {
			d.Close()
		}
		if d1 != nil {
			d1.Close()
		}
	}

	for {
		response, err := muxToDevice.ReadMessage()
		if err != nil {
			cleanup()
			return
		}

		decoder := plist.NewDecoder(bytes.NewReader(response.Payload))
		var message ios.AttachedMessage
		err = decoder.Decode(&message)
		if err != nil {
			log.Info("Failed decoding MuxMessage", message, err)
			cleanup()
			return
		}

		needSend := false
		switch message.MessageType {
		case ListenMessageAttached:
			if message.Properties.SerialNumber == serial {
				needSend = true
				deviceId = message.DeviceID
				break
			}
		case ListenMessageDetached:
			if message.DeviceID == deviceId {
				needSend = true
			}
		case ListenMessagePaired:
			if message.DeviceID == deviceId {
				needSend = true
			}
		default:
			log.Fatalf("unknown listen message type: ")
		}

		if needSend {
			err = muxOnUnixSocket.SendMuxMessage(response)
		}
	}
}

func proxyMuxConnection(muxOnUnixSocket *ios.UsbMuxConnection) {
	for {
		request, err := muxOnUnixSocket.ReadMessage()
		if err != nil {
			muxOnUnixSocket.ReleaseDeviceConnection().Close()
			if err == io.EOF {
				log.Errorf("EOF")
				return
			}
			log.Info("Failed reading UsbMuxMessage", err)
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

func handleConnection(conn net.Conn) {
	connListeningOnUnixSocket := ios.NewUsbMuxConnection(ios.NewDeviceConnectionWithConn(conn))

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered a panic: %v", r)
				debug.PrintStack()
				os.Exit(1)
				return
			}
		}()
		proxyMuxConnection(connListeningOnUnixSocket)
	}()
}
