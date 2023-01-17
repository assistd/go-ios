package ioskit

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type PhoneService struct {
	ServicePort uint16
	ServiceName string
	UseSSL      bool
}

type decoder interface {
	decode([]byte)
}
type serviceConfig struct {
	codec            func(string, string, *log.Entry) decoder
	handshakeOnlySSL bool
}

var serviceConfigurations = map[string]serviceConfig{
	"com.apple.instruments.remoteserver":                 {nil, true},
	"com.apple.accessibility.axAuditDaemon.remoteserver": {nil, true},
	"com.apple.testmanagerd.lockdown":                    {nil, true},
	"com.apple.debugserver":                              {nil, true},
}

func swapUint16(port uint16) uint16 {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, port)
	return binary.BigEndian.Uint16(buf)
}

// Proxy 修改自binforwad.handleConnectToService
func (s *PhoneService) Proxy(p *Provider) error {
	logger := log.WithField("service-port", s.ServicePort)

	socket := p.socket
	pos := strings.Index(socket, ":")
	portToListen := swapUint16(s.ServicePort)
	logger.Infof("service:%v, port %v -> %v", s.ServiceName, s.ServicePort, portToListen)
	addr := socket[0:pos] + ":" + strconv.Itoa(int(s.ServicePort))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	_, shakeOnly := serviceConfigurations[s.ServiceName]
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("error with connection: %e", err)
			return err
		}
		deviceConn := ios.NewDeviceConnectionWithConn(conn)

		logger.Infof("service:%v, new connection:%v", s.ServiceName, conn)

		// for mux to device
		netConn, err := p.device.NewConn(nil)
		if err != nil {
			return fmt.Errorf("connect to device's usbmuxd failed:%v", err)
		}
		deviceConn2 := ios.NewDeviceConnectionWithConn(netConn)
		usbmuxConn2 := ios.NewUsbMuxConnection(deviceConn2)
		err = usbmuxConn2.Connect(p.deviceID, s.ServicePort)
		if err != nil {
			conn.Close()
			netConn.Close()
			return err
		}

		if s.UseSSL {
			if shakeOnly {
				deviceConn.EnableSessionSslServerModeHandshakeOnly(p.pairRecord)
				deviceConn2.EnableSessionSslHandshakeOnly(p.pairRecord)
			} else {
				deviceConn.EnableSessionSslServerMode(p.pairRecord)
				deviceConn2.EnableSessionSsl(p.pairRecord)
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
	}
}
