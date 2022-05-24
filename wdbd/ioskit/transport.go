package ioskit

import (
	"bytes"
	"context"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
	"io"
	"net"
	"sync"
)

type Transport struct {
	socket     string
	clientConn net.Conn
	mutex      sync.Mutex
}

// NewTransport init transport
func NewTransport(socket string, clientConn net.Conn) *Transport {
	return &Transport{
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
	t.proxyMuxConnection(clientMuxConn)
	//ctx, cancel := context.WithCancel(context.Background())
}

func (t *Transport) proxyMuxConnection(muxOnUnixSocket *ios.UsbMuxConnection) {
	var muxToDevice *IosMuxConn

	for {
		request, err := muxOnUnixSocket.ReadMessage()
		if err != nil {
			inConn := muxOnUnixSocket.ReleaseDeviceConnection()
			if inConn != nil {
				inConn.Close()
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

		log.Infof("transport: read UsbMuxMessage:%v", decodedRequest)

		messageType := decodedRequest["MessageType"]
		switch messageType {
		case MuxMessageTypeListen:
			t.handleListen(muxOnUnixSocket)
			return
		case MuxMessageTypeConnect:
			deviceId := decodedRequest["DeviceID"].(uint64)
			remoteDevice, err := globalUsbmuxd.GetRemoteDeviceById(int(deviceId))
			if err != nil {
				log.Fatalln("unknown serial: ", err)
			}

			if muxToDevice == nil {
				devStream, err := remoteDevice.NewConn(context.Background())
				if err != nil {
					log.Errorf("transport: connect to %v failed: %v", t.socket, err)
					muxOnUnixSocket.Close()
					return
				}

				muxToDevice = &IosMuxConn{
					conn: devStream,
				}
			}

			log.Infof("CONNECT: replace DeviceId: %v -> %v", decodedRequest, remoteDevice.GetIOSDeviceId())
			decodedRequest["DeviceID"] = remoteDevice.GetIOSDeviceId()
			err = muxToDevice.Send(decodedRequest)
			if err != nil {
				log.Errorf("transport: failed write to device: %v", err)
				muxOnUnixSocket.Close()
				break
			}
			t.handleConnect(context.Background(), muxToDevice.conn)
			return
		case MuxMessageTypeListDevices:
			//TODO: usbmuxd允许在单个connection中多次执行ListDevices指令，待写测试代码确认，所以这里不直接返回
			t.handleListDevices(muxOnUnixSocket)
		case MuxMessageTypeListListeners:
			log.Fatalf("not supported %v yet", MuxMessageTypeListListeners)
		case MuxMessageTypeReadBUID:
			log.Fatalf("not supported %v yet", MuxMessageTypeReadBUID)
		case MuxMessageTypeReadPairRecord:
			fallthrough
		case MuxMessageTypeSavePairRecord:
			fallthrough
		case MuxMessageTypeDeletePairRecord:
			// readpairrecord.go#ReadPair
			pairId := decodedRequest["PairRecordID"].(string)
			remoteDevice, err := globalUsbmuxd.GetRemoteDevice(pairId)
			if err != nil {
				log.Fatalln("unknown serial: ", err)
			}

			if muxToDevice == nil {
				devStream, err := remoteDevice.NewConn(context.Background())
				if err != nil {
					log.Errorf("transport: connect to %v failed: %v", t.socket, err)
					muxOnUnixSocket.Close()
					return
				}

				muxToDevice = &IosMuxConn{
					conn: devStream,
				}
			}

			err = muxToDevice.SendMuxMessage(request)
			if err != nil {
				log.Errorf("transport: failed write to device: %v", err)
				muxOnUnixSocket.Close()
				break
			}

			response, err := muxToDevice.ReadMessage()
			err = muxOnUnixSocket.SendMuxMessage(response)
			if err != nil {
				log.Errorf("transport: failed write to client: %v", err)
				// 重复close应该没啥问题
				muxOnUnixSocket.Close()
				break
			}
		default:
			log.Fatalf("Unexpected command %s received!", messageType)
		}
	}
}

func (t *Transport) handleListDevices(muxOnUnixSocket *ios.UsbMuxConnection) {
	list := make([]ios.DeviceEntry, len(globalUsbmuxd.registry.Devices()))
	for i, d := range globalUsbmuxd.registry.Devices() {
		list[i] = ios.DeviceEntry(d)
	}

	deviceList := ios.DeviceList{
		DeviceList: list,
	}
	err := muxOnUnixSocket.Send(deviceList)
	if err != nil {
		log.Errorln("transport: LISTEN: write failed:", err)
	}
}

func (t *Transport) handleConnect(ctx context.Context, devConn io.ReadWriter) {
	closed := false
	ctx2, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		io.Copy(devConn, t.clientConn)
		if ctx2.Err() == nil {
			cancel()
			t.clientConn.Close()
			closed = true
		}

		log.Errorf("transport: CONNECT: forward: close clientConn <-- deviceConn")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		io.Copy(t.clientConn, devConn)
		if ctx2.Err() == nil {
			cancel()
			t.clientConn.Close()
			closed = true
		}

		log.Errorf("forward: close clientConn --> deviceConn")
		wg.Done()
	}()

	<-ctx2.Done()
	if !closed {
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

	onAdd := func(ctx context.Context, d wdbd.DeviceEntry) {
		d.MessageType = ListenMessageAttached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			log.Errorln("transport: LISTEN: write failed:", err)
			cleanup()
		}
	}

	onRemove := func(ctx context.Context, d wdbd.DeviceEntry) {
		d.MessageType = ListenMessageDetached
		err := muxOnUnixSocket.Send(d)
		if err != nil {
			log.Errorln("transport: LISTEN: write failed:", err)
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
		log.Errorln("transport: LISTEN: write failed:", err)
		cleanup()
		return
	}

	// trigger onAdd/onRemove
	for _, d := range globalUsbmuxd.registry.Devices() {
		log.Infof("--> %+v", d)
		onAdd(nil, d)
	}
	unListen := globalUsbmuxd.registry.Listen(wdbd.NewDeviceListener(onAdd, onRemove))
	defer unListen()
	defer cleanup()

	//use this to detect when the conn is closed. There shouldn't be any messages received ever.
	_, err = muxOnUnixSocket.ReadMessage()
	log.Error("transport: LISTEN: error on read", err)
}
