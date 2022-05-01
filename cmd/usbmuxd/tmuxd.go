package main

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"math"
	"time"
)

var (
	// Registry of all the discovered devices.
	registry = NewRegistry()

	// cache is a map of device serials to fully resolved bindings.
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
func newTmuxd(addr string, port int) (*tmuxd, error) {
	/*
		client, err := adb.NewWithConfig(adb.ServerConfig{
			Host: adbAddr,
			Port: adbPort,
		})
		if err != nil {
			return nil, err
		}

		return &tmuxd {
			adb:      client,
			portBase: adbdPort,
			port:     adbdPort,
			muxMap:  make(map[string]*Usbmuxd),
		}, nil
	*/
	return nil, nil
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

func (a *tmuxd) kickWithDeviceId(devicdId int) {
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

		d := NewUsbmuxd(port, serial)
		err := d.Run()
		if err == nil {
			log.Errorln("run adbd failed: ", err)
			return
		}

		time.Sleep(time.Second / 2)
	}
}

func (a *tmuxd) run() error {
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
			message, err := attachedReceiver()
			if err != nil {
				log.Errorln("Failed decoding MuxMessage", message, err)
				deviceConn.Close()
				break
			}

			switch message.MessageType {
			case ListenMessageAttached:
				go a.spawn(message.Properties.SerialNumber, message.DeviceID)
			case ListenMessageDetached:
				a.kickWithDeviceId(message.DeviceID)
			case ListenMessagePaired:
			default:
				log.Fatalf("unknown listen message type: ")
			}
		}
	}
}
