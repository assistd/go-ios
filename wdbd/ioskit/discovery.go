package ioskit

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"time"
)

type DeviceMonitor interface {
	Monitor(ctx context.Context, r *wdbd.Registry, interval time.Duration)
}

func attachedMessageToDevice(msg ios.AttachedMessage) ios.DeviceEntry {
	return ios.DeviceEntry{
		DeviceID:   msg.DeviceID,
		Properties: msg.Properties,
	}
}

func (a *tmuxd) Monitor(ctx context.Context, r *wdbd.Registry, interval time.Duration) error {
	for {
		deviceConn, err := ios.NewDeviceConnection(a.socket)
		if err != nil {
			log.Errorf("could not connect to %s with err %+v", a.socket, err)
			return fmt.Errorf("could not connect to %s with err %v", a.socket, err)
		}

		muxConnection := ios.NewUsbMuxConnection(deviceConn)
		attachedReceiver, err := muxConnection.Listen()
		if err != nil {
			log.Errorln("tmuxd: failed issuing Listen command:", err)
			deviceConn.Close()
			continue
		}

		for {
			msg, err := attachedReceiver()
			if err != nil {
				log.Errorln("tmuxd: failed decoding MuxMessage", msg, err)
				deviceConn.Close()
				break
			}

			log.Infoln("tmuxd: ios monitor: ", msg)

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
				log.Fatalln("tmuxd: unknown listen message type: ", msg.MessageType)
			}
		}
	}
}
