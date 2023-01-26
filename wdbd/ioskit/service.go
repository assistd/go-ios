package ioskit

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type PhoneService struct {
	Port   uint16
	Name   string
	UseSSL bool
}

var serviceConfigurations = map[string]bool{
	"com.apple.instruments.remoteserver":                 true,
	"com.apple.accessibility.axAuditDaemon.remoteserver": true,
	"com.apple.testmanagerd.lockdown":                    true,
	"com.apple.debugserver":                              true,
}

// Proxy 修改自binforwad.handleConnectToService
// 每次给LockDown发StartService消息，LockDown都会返回一个新的端口，并供应用层连接，这意味着不能使用for循环持续
// 监听端口，否则会造成端口泄露。
func (s *PhoneService) Proxy(p *Provider) error {
	logger := log.WithFields(log.Fields{"port": s.Port, "name": s.Name})

	socket := p.socket
	pos := strings.Index(socket, ":")
	addr := socket[0:pos] + ":" + strconv.Itoa(int(s.Port))
	logger.Infof("service:%v, port %v -> %v", s.Name, s.Port, ios.Ntohs(s.Port))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	conn, err := listener.Accept()
	if err != nil {
		logger.Errorf("error with connection: %e", err)
		return err
	}
	logger.Infof("service:%v, new connection:%v", s.Name, conn)
	listener.Close()
	deviceConn := ios.NewDeviceConnectionWithConn(conn)

	// for mux to device
	conn2, err := p.device.NewConn(nil)
	if err != nil {
		return fmt.Errorf("connect to device's usbmuxd failed:%v", err)
	}
	deviceConn2 := ios.NewDeviceConnectionWithConn(conn2)
	usbmuxConn2 := ios.NewUsbMuxConnection(deviceConn2)
	err = usbmuxConn2.Connect(p.deviceID, s.Port)
	if err != nil {
		deviceConn.Close()
		deviceConn2.Close()
		return err
	}

	cleanup := func() {
		deviceConn.Close()
		deviceConn2.Close()
	}
	if s.UseSSL {
		if _, ok := serviceConfigurations[s.Name]; ok {
			if err := deviceConn.EnableSessionSslServerModeHandshakeOnly(p.pairRecord); err != nil {
				logger.Errorln("ssl fail:", err)
				cleanup()
				return err
			}
			if err := deviceConn2.EnableSessionSslHandshakeOnly(p.pairRecord); err != nil {
				logger.Errorln("ssl fail:", err)
				cleanup()
				return err
			}
		} else {
			if err := deviceConn.EnableSessionSslServerMode(p.pairRecord); err != nil {
				logger.Errorln("ssl fail:", err)
				cleanup()
				return err
			}
			if err := deviceConn2.EnableSessionSsl(p.pairRecord); err != nil {
				logger.Errorln("ssl fail:", err)
				cleanup()
				return err
			}
		}
	}

	go func() {
		io.Copy(deviceConn.Writer(), deviceConn2.Reader())
		deviceConn.Close()
	}()
	go func() {
		io.Copy(deviceConn2.Writer(), deviceConn.Reader())
		deviceConn2.Close()
	}()
	return nil
}
