package ioskit

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

var transportId int32

type Transport struct {
	id         int32
	socket     string
	clientConn net.Conn
	logger     *log.Entry
}

// NewTransport init transport
func NewTransport(socket string, clientConn net.Conn) *Transport {
	transportId++
	return &Transport{
		id:         transportId,
		socket:     socket,
		clientConn: clientConn,
		logger:     log.WithField("id", transportId),
	}
}

// Kick kick off the remote adb server's connection
func (t *Transport) Kick() {
}

// HandleLoop run adb packet reading and writing loop
func (t *Transport) HandleLoop() {
	clientMuxConn := ios.NewUsbMuxConnection(ios.NewDeviceConnectionWithConn(t.clientConn))
	t.proxyMuxConnection(clientMuxConn)
	//ctx, cancel := context.WithCancel(context.Background())
}

func sendRequest(conn net.Conn, message ios.UsbMuxMessage) error {
	err := binary.Write(conn, binary.LittleEndian, message.Header)
	if err != nil {
		return err
	}

	_, err = conn.Write(message.Payload)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transport) proxyMuxConnection(muxOnUnixSocket *ios.UsbMuxConnection) {
	var muxToDevice net.Conn

	for {
		request, err := muxOnUnixSocket.ReadMessage()
		if err != nil {
			muxOnUnixSocket.Close()
			t.logger.Errorln("transport: failed reading UsbMuxMessage", err)
			return
		}

		var decodedRequest map[string]interface{}
		decoder := plist.NewDecoder(bytes.NewReader(request.Payload))
		err = decoder.Decode(&decodedRequest)
		if err != nil {
			muxOnUnixSocket.Close()
			t.logger.Fatalln("transport: failed decoding MuxMessage", request, err)
			return
		}

		t.logger.Infof("transport %v: read UsbMuxMessage:%v", t.id, decodedRequest)

		messageType := decodedRequest["MessageType"]
		switch messageType {
		case MuxMessageTypeListDevices:
			// NOTE: usbmuxd允许在单个connection中多次执行ListDevices指令，待写测试代码确认，所以这里不直接返回
			t.handleListDevices(muxOnUnixSocket)
		case MuxMessageTypeListen:
			t.handleListen(muxOnUnixSocket)
			return

		case MuxMessageTypeListListeners:
			fallthrough
		case MuxMessageTypeReadBUID:
			fallthrough
		case MuxMessageTypeReadPairRecord:
			fallthrough
		case MuxMessageTypeSavePairRecord:
			fallthrough
		case MuxMessageTypeDeletePairRecord:
			// readpairrecord.go#ReadPair
			// pairId := decodedRequest["PairRecordID"].(string)
			fallthrough
		case MuxMessageTypeConnect:
			remoteDevice := globalUsbmuxd.GetRemoteDevice()
			if muxToDevice == nil {
				devStream, err := remoteDevice.NewConn(nil)
				if err != nil {
					t.logger.Errorf("transport: connect to %v failed: %v", t.socket, err)
					muxOnUnixSocket.Close()
					return
				}
				muxToDevice = devStream
			}

			err = sendRequest(muxToDevice, request)
			if err != nil {
				t.logger.Errorf("transport: failed write to device: %v", err)
				muxOnUnixSocket.Close()
				muxToDevice.Close()
				return
			}
			t.forward(context.Background(), muxToDevice)
			return
		default:
			t.logger.Fatalf("Unexpected command %s received!", messageType)
		}
	}
}

func (t *Transport) handleListDevices(muxOnUnixSocket *ios.UsbMuxConnection) {
	d, err := globalUsbmuxd.remote.ListDevices()
	var list []ios.DeviceEntry
	if err == nil {
		list = make([]ios.DeviceEntry, 1)
		list[0] = ios.DeviceEntry(d)
	}

	deviceList := ios.DeviceList{
		DeviceList: list,
	}
	err = muxOnUnixSocket.Send(deviceList)
	if err != nil {
		t.logger.Errorln("transport: list-device write to client failed:", err)
	}
}

func (t *Transport) forward(ctx context.Context, devConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		io.Copy(devConn, t.clientConn)
		devConn.Close()
		t.logger.Errorf("forward: close clientConn <-- deviceConn")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		io.Copy(t.clientConn, devConn)
		t.clientConn.Close()
		t.logger.Errorf("forward: close clientConn --> deviceConn")
		wg.Done()
	}()

	wg.Wait()
}

func (t *Transport) handleListen(muxOnUnixSocket *ios.UsbMuxConnection) {
	cleanup := func() {
		muxOnUnixSocket.Close()
	}

	onAdd := func(ctx context.Context, d DeviceEntry) {
		d.MessageType = ListenMessageAttached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			t.logger.Errorln("transport: LISTEN: write failed:", err)
			cleanup()
		}
	}

	onRemove := func(ctx context.Context, d DeviceEntry) {
		d.MessageType = ListenMessageDetached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			t.logger.Errorln("transport: LISTEN: write failed:", err)
			cleanup()
		}
	}
	// TODO: support ListenMessagePaired

	// send Listen response
	resp := &ios.MuxResponse{
		MessageType: "Result",
		Number:      0,
	}
	err := muxOnUnixSocket.Send(resp)
	if err != nil {
		t.logger.Errorln("transport: LISTEN: write failed:", err)
		cleanup()
		return
	}

	// trigger onAdd/onRemove
	d, err := globalUsbmuxd.remote.ListDevices()
	if err == nil {
		t.logger.Infof("--> %+v", d)
		onAdd(nil, d)
	}

	unListen := globalUsbmuxd.remote.registry.Listen(NewDeviceListener(onAdd, onRemove))
	defer unListen()
	defer cleanup()

	//use this to detect when the conn is closed. There shouldn't be any messages received ever.
	_, err = muxOnUnixSocket.ReadMessage()
	t.logger.Error("transport: LISTEN: error on read", err)
}
