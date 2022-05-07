package ioskit

import (
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
)

var (
	globalUsbmuxd *Usbmuxd
)

type Usbmuxd struct {
	registry          *wdbd.Registry
	socket            string
	transports        map[*Transport]struct{}
	mutex             sync.Mutex
	deviceId          int
	deviceSerialIdMap map[string]int
	muxMap            map[string]*RemoteDevice // addr -> RemoteDevice
	serialMap         map[string]*RemoteDevice // serial -> RemoteDevice
}

// NewUsbmuxd create an Usbmuxd instance
func NewUsbmuxd(socket string) *Usbmuxd {
	return &Usbmuxd{
		registry:          wdbd.NewRegistry(),
		socket:            socket,
		transports:        make(map[*Transport]struct{}),
		deviceSerialIdMap: make(map[string]int),
		muxMap:            make(map[string]*RemoteDevice),
		serialMap:         make(map[string]*RemoteDevice),
	}
}

// ListenAddr return inner serving port
func (a *Usbmuxd) ListenAddr() string {
	return a.socket
}

// Kick stop all transport of this Adbd instance
func (a *Usbmuxd) Kick() {
	for transport, _ := range a.transports {
		transport.Kick()
	}
}

func (a *Usbmuxd) listAll() string {
	var out string
	for serial, proxy := range a.muxMap {
		out += fmt.Sprintf("%s:%s\n", serial, proxy)
	}

	return out
}

func (a *Usbmuxd) Add(device *RemoteDevice) error {
	key := fmt.Sprintf("%s", device.Addr)
	a.muxMap[key] = device
	a.serialMap[device.Serial] = device
	return nil
}

func (a *Usbmuxd) Remove(device RemoteDevice) error {
	key := fmt.Sprintf("%s", device.Addr)
	if _, ok := a.muxMap[key]; ok {
		delete(a.muxMap, key)
	}

	return nil
}

func (a *Usbmuxd) GetRemoteDevice(serial string) (*RemoteDevice, error) {
	if d, ok := a.serialMap[serial]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("device not found: %v", serial)
}

func (a *Usbmuxd) GetRemoteDeviceById(deviceId int) (*RemoteDevice, error) {
	entry, err := a.registry.Device(deviceId)
	if err != nil {
		return nil, err
	}
	return a.GetRemoteDevice(entry.Properties.SerialNumber)
}

// Run serve a tcp server, and do the message switching between remote usbmuxd and local one
func (a *Usbmuxd) Run() error {
	//listener, err := net.Listen("tcp", fmt.Sprintf(":%v", a.port))
	listener, err := net.Listen("tcp", a.socket)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", a.socket, err)
	}

	log.Debugln("listen on: ", a.socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		t := NewTransport(a.socket, conn)
		a.mutex.Lock()
		a.transports[t] = struct{}{}
		a.mutex.Unlock()

		log.Debugln("usbmuxd: new transport: ", t)
		go func() {
			t.HandleLoop()

			a.mutex.Lock()
			delete(a.transports, t)
			a.mutex.Unlock()
		}()
	}
}