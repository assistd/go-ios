package ioskit

import (
	"context"
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"
)

var (
	// Registry of all the discovered devices.
	registry = wdbd.NewRegistry()

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

// NewTmuxd init adb kit
func NewTmuxd(socket string, port int) (*tmuxd, error) {
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
		a.muxMap[serial] = d

		err := d.Run()
		if err == nil {
			log.Errorln("tmuxd: run failed: ", err)
			return
		}

		time.Sleep(time.Second / 2)
	}
}

func (a *tmuxd) listen() {
	registry.Listen(wdbd.NewDeviceListener(
		func(ctx context.Context, device wdbd.DeviceEntry) {
			go a.spawn(device.Properties.SerialNumber, device.DeviceID)
		}, func(ctx context.Context, device wdbd.DeviceEntry) {
			a.kick(device.Properties.SerialNumber)
		},
	))
}
