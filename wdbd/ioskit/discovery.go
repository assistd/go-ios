package ioskit

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

var (
	// cache is a map of device serials to fully resolved bindings.
	cacheMutex sync.Mutex
	cancelMap  = map[int]Ctx{}
)

type DeviceMonitor interface {
	Monitor(ctx context.Context, r *Registry, interval time.Duration)
}

type IOSDeviceMonitor struct {
	Network string
	Addr    string
}

// NewDeviceMonitor init adb kit
func NewDeviceMonitor(network, addr string) (*IOSDeviceMonitor, error) {
	return &IOSDeviceMonitor{
		Network: network,
		Addr:    addr,
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

func (a *IOSDeviceMonitor) Monitor(ctx context.Context, r *Registry, serial string, interval time.Duration) error {
	deviceSerial := make(map[int]string)
	for {
		c, err := net.Dial(a.Network, a.Addr)
		if err != nil {
			return err
		}
		deviceConn := ios.NewDeviceConnectionWithConn(c)
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

				// usbmuxd on Remote's macOS may restart, all DeviceID will be resigned.
				d, err := r.DeviceBySerial(serial)
				if err != nil {
					cacheMutex.Lock()
					dCtx := cancelMap[d.DeviceID]
					r.RemoveDevice(dCtx.ctx, d)
					dCtx.cancel()
					cacheMutex.Unlock()
				}
				break
			}
			log.Infoln("ios-monitor: msg: ", msg)

			switch msg.MessageType {
			case ListenMessageAttached:
				if msg.Properties.SerialNumber != serial {
					continue
				}
				deviceSerial[msg.DeviceID] = msg.Properties.SerialNumber
				dCtx, cancel := context.WithCancel(ctx)

				cacheMutex.Lock()
				cancelMap[msg.DeviceID] = Ctx{
					cancel: cancel,
					ctx:    dCtx,
				}
				cacheMutex.Unlock()
				r.AddDevice(dCtx, DeviceEntry(attachedMessageToDevice(msg)))
			case ListenMessageDetached:
				msg.Properties.SerialNumber = deviceSerial[msg.DeviceID]
				if msg.Properties.SerialNumber != serial {
					continue
				}
				cacheMutex.Lock()
				dCtx, _ := cancelMap[msg.DeviceID]
				r.RemoveDevice(dCtx.ctx, DeviceEntry(attachedMessageToDevice(msg)))
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
