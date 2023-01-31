package ioskit

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

type Usbmuxd struct {
	socket     string
	remote     *RemoteDevice // serial -> RemoteDevice
	transports map[*Transport]struct{}
	mutex      sync.Mutex
}

// NewUsbmuxd create an Usbmuxd instance
func NewUsbmuxd(socket string, device *RemoteDevice) *Usbmuxd {
	return &Usbmuxd{
		socket:     socket,
		transports: make(map[*Transport]struct{}),
		remote:     device,
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

func (a *Usbmuxd) GetRemoteDevice() *RemoteDevice {
	return a.remote
}

// Run serve a tcp server, and do the message switching between remote usbmuxd and local one
func (a *Usbmuxd) Run() error {
	pos := strings.Index(a.socket, ":")
	if pos < 0 {
		return fmt.Errorf("invalid socket: %v", a.socket)
	}

	network, addr := a.socket[0:pos], a.socket[pos+1:]
	listener, err := net.Listen(network, addr)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", a.socket, err)
	}

	if network == "unix" {
		os.Chmod(addr, 0777)
	}
	cleanup := func() {
		if network == "unix" {
			os.Remove(addr)
		}
	}

	defer cleanup()

	log.Debugln("listen on: ", a.socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		t := NewTransport(conn, a.remote)
		a.mutex.Lock()
		a.transports[t] = struct{}{}
		a.mutex.Unlock()

		log.Debugln("usbmuxd: new transport: ", t)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Recovered a panic: %v", r)
					cleanup()
					debug.PrintStack()
					os.Exit(1)
					return
				}
			}()

			t.HandleLoop()

			a.mutex.Lock()
			delete(a.transports, t)
			a.mutex.Unlock()
		}()
	}
}
