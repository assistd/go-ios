package ioskit

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
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

func attachedMessageToDevice(msg ios.AttachedMessage) ios.DeviceEntry {
	return ios.DeviceEntry{
		DeviceID:   msg.DeviceID,
		Properties: msg.Properties,
	}
}

func (a *IOSDeviceMonitor) Monitor(ctx context.Context, r *Registry, serial string, interval time.Duration) error {
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
				r.RemoveAll()
				break
			}
			log.Infoln("ios-monitor: msg: ", msg)

			switch msg.MessageType {
			case ListenMessageAttached:
				if msg.Properties.SerialNumber != serial {
					continue
				}
				r.AddDevice(ctx, DeviceEntry(attachedMessageToDevice(msg)))
			case ListenMessageDetached:
				r.RemoveDevice(DeviceEntry(attachedMessageToDevice(msg)))
			case ListenMessagePaired:
				// TODO:
			default:
				log.Fatalln("ios-monitor: unknown listen message type: ", msg.MessageType)
			}
		}
	}
}
