package ioskit

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type PowerAssertionService PhoneService

func (s *PowerAssertionService) Proxy(p *Provider) error {
	logger := log.WithFields(log.Fields{"port": s.Port, "name": s.Name})

	socket := p.socket
	pos := strings.Index(p.socket, ":")
	addr := socket[0:pos] + ":" + strconv.Itoa(int(s.Port))
	logger.Infof("aa_service:%v, port %v", s.Name, s.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("aa_service error with connection: %e", err)
			return err
		}
		logger.Infof("aa_service:%v, new connection:%v", s.Name, conn)
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

		if s.UseSSL {
			if _, ok := serviceConfigurations[s.Name]; ok {
				deviceConn.EnableSessionSslServerModeHandshakeOnly(p.pairRecord)
				deviceConn2.EnableSessionSslHandshakeOnly(p.pairRecord)
			} else {
				deviceConn.EnableSessionSslServerMode(p.pairRecord)
				deviceConn2.EnableSessionSsl(p.pairRecord)
			}
		}

		codec := ios.NewPlistCodec()
		go func() {
			defer deviceConn2.Close()
			msgbuf, err := codec.Decode(deviceConn2.Reader())
			if err != nil {
				logger.Errorf("aa_service:%v <-- failed:%v", s.Name, err)
				return
			}

			msg, err := ios.ParsePlist(msgbuf)
			if err != nil {
				logger.Errorf("aa_service:%v <-- failed:%v", s.Name, err)
				return
			}

			logger.Infof("aa_service:%v <-- %v", s.Name, msg)
			msgbuf2, _ := codec.Encode(msg)
			if _, err := deviceConn.Writer().Write(msgbuf2); err != nil {
				deviceConn.Close()
			}
			// io.Copy(deviceConn.Writer(), deviceConn2.Reader())
		}()
		go func() {
			defer deviceConn.Close()
			msgbuf, err := codec.Decode(deviceConn.Reader())
			if err != nil {
				logger.Errorf("aa_service:%v --> failed:%v", s.Name, err)
				return
			}

			msg, err := ios.ParsePlist(msgbuf)
			if err != nil {
				logger.Errorf("aa_service:%v --> failed:%v", s.Name, err)
				return
			}

			logger.Infof("aa_service:%v --> %v", s.Name, msg)
			msgbuf2, _ := codec.Encode(msg)
			if _, err := deviceConn2.Writer().Write(msgbuf2); err != nil {
				deviceConn2.Close()
			}
			// io.Copy(deviceConn2.Writer(), deviceConn.Reader())
		}()
	}
}
