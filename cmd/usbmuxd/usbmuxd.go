package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
)

type Usbmuxd struct {
	port       int
	serial     string
	transports map[*Transport]struct{}
	mutex      sync.Mutex
}

// NewUsbmuxd create an Usbmuxd instance
func NewUsbmuxd(port int, serial string) *Usbmuxd {
	return &Usbmuxd{
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
		conn, err := listener.AcceptTCP()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		t := NewTransport(conn, serial)
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
