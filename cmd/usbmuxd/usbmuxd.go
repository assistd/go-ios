package main

import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
)

type Usbmuxd struct {
	socket     string
	port       int
	serial     string
	transports map[*Transport]struct{}
	mutex      sync.Mutex
}

// NewUsbmuxd create an Usbmuxd instance
func NewUsbmuxd(port int, socket string, serial string) *Usbmuxd {
	return &Usbmuxd{
		socket: socket,
		port:   port,
		serial: serial,
	}
}

// Port return inner serving port
func (a *Usbmuxd) Port() int {
	return a.port
}

// Kick stop all transport of this Adbd instance
func (a *Usbmuxd) Kick() {
	for transport, _ := range a.transports {
		transport.Kick()
	}
}

// Run serve a tcp server, and do the message switching between remote usbmuxd and local one
func (a *Usbmuxd) Run() error {
	var listener *net.TCPListener
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%v", a.port))
	if err != nil {
		log.Errorf("fail to resolve port:%v, error:%v", a.port, err)
		return err
	}
	listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", a.port, err)
	}

	log.Debugln("listen on: ", a.port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		devConn, err := ios.NewDeviceConnection(a.socket)
		if err != nil {
			// should never be here
			devConn.Close()
			// close other transport
			a.Kick()
			listener.Close()
			log.Errorf("usbmuxd: connect to %v failed: %v", a.socket, err)
			return fmt.Errorf("usbmuxd: connect to %v failed: %v", a.socket, err)
		}

		t := NewTransport(devConn, conn, a.serial)
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
