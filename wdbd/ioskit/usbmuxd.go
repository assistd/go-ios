package ioskit

import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/prife/keepaliveconn"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type Usbmuxd struct {
	socket     string
	remote     *RemoteDevice // serial -> RemoteDevice
	transports map[*Transport]struct{}
	mutex      sync.Mutex
	keepAlive  bool
}

// NewUsbmuxd create an Usbmuxd instance
func NewUsbmuxd(socket string, keepAlive bool, device *RemoteDevice) *Usbmuxd {
	return &Usbmuxd{
		socket:     socket,
		transports: make(map[*Transport]struct{}),
		remote:     device,
		keepAlive:  keepAlive,
	}
}

// create usbmuxd listener
func (a *Usbmuxd) Listener() (net.Listener, error) {
	pos := strings.Index(a.socket, ":")
	if pos < 0 {
		return nil, fmt.Errorf("invalid socket: %v", a.socket)
	}

	network, addr := a.socket[0:pos], a.socket[pos+1:]
	if network == "unix" && runtime.GOOS != "windows" {
		if fileInfo, _ := os.Stat(ios.DefaultUsbmuxdSocket); fileInfo != nil {
			bak := fmt.Sprintf("%v.bak", ios.DefaultUsbmuxdSocket)
			_ = os.Rename(ios.DefaultUsbmuxdSocket, bak)
		}
		os.Chmod(addr, 0777)
	}
	listener, err := net.Listen(network, addr)
	if err != nil {
		return nil, fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", a.socket, err)
	}
	return listener, nil
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

// forward local tcp to remote KeepAliveTcp
func (a *Usbmuxd) Forward() error {
	listener, err := a.Listener()
	if err != nil {
		log.Errorf("create usbmuxd listener failed:%v", err)
		return fmt.Errorf("create usbmuxd listener failed:%v", err)
	}
	for {
		localConn, err := listener.Accept()
		if err != nil {
			log.Errorf("accept usbmuxd conn failed:%v", err)
			continue
		}
		go func() {
			remoteConn, err := net.Dial("tcp", a.remote.Addr)
			if err != nil {
				log.Errorf("dial remote device=[%+v] failed:%v", a.remote, err)
				return
			}
			a.copy(localConn, remoteConn)
		}()
	}
}

func (a *Usbmuxd) copy(local, remote net.Conn) {
	log.Infof("start to iocopy [%v]->[%v]", local.RemoteAddr().String(), local.RemoteAddr().String())
	if a.keepAlive {
		remote = keepaliveconn.New(remote, time.Hour)
	}
	go func() {
		defer func() {
			_ = local.Close()
		}()
		var err error
		if a.keepAlive {
			_, err = keepaliveconn.Copy(local, remote.(*keepaliveconn.KeepaliveConn))
		} else {
			_, err = io.Copy(local, remote)
		}
		if err != nil {
			log.Errorf("io copy remoteConn->localConn failed:%v", err)
			return
		}
	}()

	go func() {
		defer func() {
			_ = remote.Close()
		}()
		_, err := io.Copy(remote, local)
		if err != nil {
			log.Errorf("io copy localConn->remoteConn failed:%v", err)
			return
		}
	}()
}

// Run serve a tcp server, and do the message switching between remote usbmuxd and local one
func (a *Usbmuxd) Run() error {

	listener, err := a.Listener()
	if err != nil {
		return fmt.Errorf("create usbmuxd listener failed:%v", err)
	}

	log.Debugln("listen on: ", a.socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}
		t := NewTransport(conn, a.remote)
		if a.keepAlive {
			kpConn := keepaliveconn.New(conn, time.Hour)
			t = NewTransport(kpConn, a.remote)
		}
		a.mutex.Lock()
		a.transports[t] = struct{}{}
		a.mutex.Unlock()

		log.Debugln("usbmuxd: new transport: ", t)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Recovered a panic: %v", r)
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
