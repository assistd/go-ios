package main

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"
)

var (
	// Registry of all the discovered devices.
	registry = NewRegistry()

	// cache is a map of device serials to fully resolved bindings.
	cacheMutex   sync.Mutex
	cancelMap    = map[int]Ctx{}
	deviceSerial = make(map[int]string)
)

type Ctx struct {
	cancel context.CancelFunc
	ctx    context.Context
}

type tmuxd struct {
	socket   string
	portBase int
	port     int
	muxMap   map[string]*Usbmuxd
}

// newTmuxd init adb kit
func newTmuxd(socket string, port int) (*tmuxd, error) {
	return &tmuxd{
		socket:   socket,
		portBase: port,
		port:     port,
		muxMap:   make(map[string]*Usbmuxd),
	}, nil
}

func (a *tmuxd) nextPort() int {
	a.port = a.port + 1
	return a.port
}

func (a *tmuxd) listAll() string {
	var out string
	for serial, proxy := range a.muxMap {
		out += fmt.Sprintf("%s:%d\n", serial, proxy.Port())
	}

	return out
}

func (a *tmuxd) get(serial string) (*Usbmuxd, error) {
	if proxy, ok := a.muxMap[serial]; ok {
		return proxy, nil
	}

	return nil, fmt.Errorf("invalid serial")
}

func (a *tmuxd) kick(serial string) {
	if d, ok := a.muxMap[serial]; ok {
		d.Kick()
		delete(a.muxMap, serial)
	}
}

// spawn find a free port to spawn
func (a *tmuxd) spawn(serial string, deviceId int) {
	for {
		port := a.nextPort()
		if port > math.MaxUint16 {
			port = a.portBase
		}

		d := NewUsbmuxd(port, a.socket, serial)
		err := d.Run()
		if err == nil {
			log.Errorln("run adbd failed: ", err)
			return
		}

		time.Sleep(time.Second / 2)
	}
}

func attachedMessageToDevice(msg ios.AttachedMessage) ios.DeviceEntry {
	return ios.DeviceEntry{
		DeviceID:   msg.DeviceID,
		Properties: msg.Properties,
	}
}

func (a *tmuxd) listen() {
	registry.Listen(NewDeviceListener(
		func(ctx context.Context, device Device) {
			go a.spawn(device.Properties.SerialNumber, device.DeviceID)
		}, func(ctx context.Context, device Device) {
			a.kick(device.Properties.SerialNumber)
		},
	))
}

func (a *tmuxd) run(ctx context.Context) error {
	for {
		deviceConn, err := ios.NewDeviceConnection(a.socket)
		if err != nil {
			log.Errorf("could not connect to %s with err %+v", a.socket, err)
			return fmt.Errorf("could not connect to %s with err %v", a.socket, err)
		}

		muxConnection := ios.NewUsbMuxConnection(deviceConn)
		attachedReceiver, err := muxConnection.Listen()
		if err != nil {
			log.Errorln("failed issuing Listen command:", err)
			deviceConn.Close()
			continue
		}

		for {
			msg, err := attachedReceiver()
			if err != nil {
				log.Errorln("Failed decoding MuxMessage", msg, err)
				deviceConn.Close()
				break
			}

			switch msg.MessageType {
			case ListenMessageAttached:
				deviceSerial[msg.DeviceID] = msg.Properties.SerialNumber
				dCtx, cancel := context.WithCancel(ctx)

				cacheMutex.Lock()
				cancelMap[msg.DeviceID] = Ctx{
					cancel: cancel,
					ctx:    dCtx,
				}
				cacheMutex.Unlock()
				registry.AddDevice(dCtx, Device(attachedMessageToDevice(msg)))
			case ListenMessageDetached:
				msg.Properties.SerialNumber = deviceSerial[msg.DeviceID]
				cacheMutex.Lock()
				dCtx, _ := cancelMap[msg.DeviceID]
				registry.RemoveDevice(dCtx.ctx, Device(attachedMessageToDevice(msg)))
				dCtx.cancel()
				cacheMutex.Unlock()
			case ListenMessagePaired:
				// TODO:
			default:
				log.Fatalf("unknown listen message type: ")
			}
		}
	}
}
