package ioskit

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var (
	// cache is a map of device serials to fully resolved bindings.
	cacheMutex   sync.Mutex
	cancelMap    = map[int]Ctx{}
	deviceSerial = make(map[int]string)
)

type DeviceMonitor interface {
	Monitor(ctx context.Context, r *wdbd.Registry, interval time.Duration)
}

type IOSDeviceMonitor struct {
	Network string
	Addr    string
}

// NewDeviceMonitor init adb kit
func NewDeviceMonitor(socket string) (*IOSDeviceMonitor, error) {
	return &IOSDeviceMonitor{
		Network: "unix",
		Addr:    socket,
	}, nil
}

type Ctx struct {
	cancel context.CancelFunc
	ctx    context.Context
}

func attachedMessageToDevice(msg ios.AttachedMessage) ios.DeviceEntry {
	return ios.DeviceEntry{
		DeviceID:   msg.DeviceID,
		Properties: msg.Properties,
	}
}

func (a *IOSDeviceMonitor) Monitor(ctx context.Context, r *wdbd.Registry, interval time.Duration) error {
	for {
		deviceConn, err := ios.NewDeviceConnection(a.Addr)
		if err != nil {
			log.Errorf("could not connect to %s with err %+v", a.Addr, err)
			return fmt.Errorf("could not connect to %s with err %v", a.Addr, err)
		}

		muxConnection := ios.NewUsbMuxConnection(deviceConn)
		attachedReceiver, err := muxConnection.Listen()
		if err != nil {
			log.Errorln("ios-monitor: failed issuing Listen command:", err)
			deviceConn.Close()
			continue
		}

		for {
			msg, err := attachedReceiver()
			if err != nil {
				log.Errorln("ios-monitor: failed decoding MuxMessage", msg, err)
				deviceConn.Close()
				break
			}

			log.Infoln("ios-monitor: ios monitor: ", msg)

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
				r.AddDevice(dCtx, wdbd.DeviceEntry(attachedMessageToDevice(msg)))
			case ListenMessageDetached:
				msg.Properties.SerialNumber = deviceSerial[msg.DeviceID]
				cacheMutex.Lock()
				dCtx, _ := cancelMap[msg.DeviceID]
				r.RemoveDevice(dCtx.ctx, wdbd.DeviceEntry(attachedMessageToDevice(msg)))
				dCtx.cancel()
				cacheMutex.Unlock()
			case ListenMessagePaired:
				// TODO:
			default:
				log.Fatalln("ios-monitor: unknown listen message type: ", msg.MessageType)
			}
		}
	}
}
