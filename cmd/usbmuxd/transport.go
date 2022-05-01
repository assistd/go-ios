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
	Serial     string
	socket     string
	clientConn net.Conn
	mutex      sync.Mutex
}

// NewTransport init transport
func NewTransport(socket string, clientConn net.Conn, serial string) *Transport {
	return &Transport{
		Serial:     serial,
		socket:     socket,
		clientConn: clientConn,
	}
}

// Kick kick off the remote adb server's connection
func (t *Transport) Kick() {
}

// HandleLoop run adb packet reading and writing loop
func (t *Transport) HandleLoop() {
	clientMuxConn := ios.NewUsbMuxConnection(ios.NewDeviceConnectionWithConn(t.clientConn))
	go t.proxyMuxConnection(clientMuxConn)
	//ctx, cancel := context.WithCancel(context.Background())
}

func (t *Transport) proxyMuxConnection(muxOnUnixSocket *ios.UsbMuxConnection) {
	var devConn *ios.DeviceConnection

	for {
		request, err := muxOnUnixSocket.ReadMessage()
		if err != nil {
			inConn := muxOnUnixSocket.ReleaseDeviceConnection()
			if inConn != nil {
				inConn.Close()
			}
			if devConn != nil {
				devConn.Close()
			}
			log.Errorln("transport: failed reading UsbMuxMessage", err)
			return
		}

		var decodedRequest map[string]interface{}
		decoder := plist.NewDecoder(bytes.NewReader(request.Payload))
		err = decoder.Decode(&decodedRequest)
		if err != nil {
			log.Fatalln("transport: failed decoding MuxMessage", request, err)
			return
		}

		messageType := decodedRequest["MessageType"]
		switch messageType {
		case MuxMessageTypeListen:
			t.handleListen(muxOnUnixSocket)
			return
		case MuxMessageTypeConnect:
			t.handleConnect(context.Background(), muxOnUnixSocket)
			return
		case MuxMessageTypeListDevices:
			//TODO: usbmuxd允许在单个connection中多次执行ListDevices指令，待写测试代码确认，所以这里不直接返回
			t.handleListDevices(muxOnUnixSocket)
		case MuxMessageTypeListListeners:
			log.Fatalf("not supported yet")
		case MuxMessageTypeReadBUID:
			fallthrough
		case MuxMessageTypeReadPairRecord:
			fallthrough
		case MuxMessageTypeSavePairRecord:
			fallthrough
		case MuxMessageTypeDeletePairRecord:
			if devConn == nil {
				devConn, err = ios.NewDeviceConnection(t.socket)
				if err != nil {
					log.Errorf("usbmuxd: connect to %v failed: %v", t.socket, err)
					muxOnUnixSocket.Close()
					return
				}
			}

			muxToDevice := ios.NewUsbMuxConnection(devConn)
			response, err := muxToDevice.ReadMessage()
			err = muxOnUnixSocket.SendMuxMessage(response)
			if err != nil {
				// 重复close应该没啥问题
				muxOnUnixSocket.Close()
				devConn.Close()
			}
		default:
			log.Fatalf("Unexpected command %s received!", messageType)
		}
	}
}

func (t *Transport) handleListDevices(muxOnUnixSocket *ios.UsbMuxConnection) {
	for _, d := range registry.Devices() {
		if d.Properties.SerialNumber != t.Serial {
			return
		}

		d.MessageType = ListenMessageAttached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			log.Errorln("transport: LISTEN: write failed:", err)
		}
	}
}

func (t *Transport) handleConnect(ctx context.Context, muxOnUnixSocket *ios.UsbMuxConnection) {
	devConn, err := ios.NewDeviceConnection(t.socket)
	if err != nil {
		log.Errorf("usbmuxd: CONNECT to %v failed: %v", t.socket, err)
		muxOnUnixSocket.Close()
		return
	}

	closed := false
	ctx2, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		io.Copy(devConn.Writer(), t.clientConn)
		if ctx2.Err() == nil {
			cancel()
			devConn.Close()
			t.clientConn.Close()
			closed = true
		}

		log.Errorf("usbmuxd: CONNECT: forward: close clientConn <-- deviceConn")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		io.Copy(t.clientConn, devConn.Reader())
		if ctx2.Err() == nil {
			cancel()
			devConn.Close()
			t.clientConn.Close()
			closed = true
		}

		log.Errorf("forward: close clientConn --> deviceConn")
		wg.Done()
	}()

	<-ctx2.Done()
	if !closed {
		devConn.Close()
		t.clientConn.Close()
	}

	wg.Wait()
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
