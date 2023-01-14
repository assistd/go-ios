package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/debugproxy"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

var transportId int32

type LockDownTransport struct {
	id     int32
	conn   net.Conn
	device *ioskit.RemoteDevice
	logger *log.Entry
	// sessionID string
	// mutex  sync.Mutex
}

func NewLockDownTransport(conn net.Conn, device *ioskit.RemoteDevice) *LockDownTransport {
	transportId++
	return &LockDownTransport{
		id:     transportId,
		conn:   conn,
		device: device,
		logger: log.WithField("id", transportId),
	}
}

func (t *LockDownTransport) ReadMessage() ([]byte, error) {
	lenbuf := make([]byte, 4)
	_, err := io.ReadFull(t.conn, lenbuf)
	if err != nil {
		return nil, err
	}
	len := binary.BigEndian.Uint32(lenbuf)

	buf := make([]byte, len)
	n, err := io.ReadFull(t.conn, buf)
	if err != nil {
		return nil, fmt.Errorf("lockdown Payload had incorrect size: %d original error: %s", n, err)
	}
	return buf, nil
}

func (t *LockDownTransport) Close() {
	t.conn.Close()
}

func (t *LockDownTransport) connectToDevice() (net.Conn, *ios.LockDownConnection, error) {
	d, err := t.device.ListDevices()
	if err != nil {
		return nil, nil, fmt.Errorf("device not existed:%v", err)
	}

	netConn, err := t.device.NewConn(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to device's usbmuxd failed:%v", err)
	}

	deviceConn := ios.NewDeviceConnectionWithConn(netConn)
	usbmuxConn := ios.NewUsbMuxConnection(deviceConn)
	lockdownToDevice, err := usbmuxConn.ConnectLockdown(d.DeviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to lockdown failed: %v", err)
	}

	return netConn, lockdownToDevice, nil
}

func (t *LockDownTransport) proxyBinaryMode() error {
	conn, lockdownToDevice, err := t.connectToDevice()
	if err != nil {
		return nil
	}
	defer lockdownToDevice.Close()

	go func() {
		io.Copy(t.conn, conn)
		t.conn.Close()
	}()

	go func() {
		io.Copy(conn, t.conn)
		conn.Close()
	}()

	return nil
}

func (t *LockDownTransport) proxyMuxConnection() error {
	_, lockdownToDevice, err := t.connectToDevice()
	if err != nil {
		return nil
	}
	defer lockdownToDevice.Close()

	for {
		request, err := t.ReadMessage()
		if err != nil {
			t.Close()
			t.logger.Errorln("client read failed", err)
			return fmt.Errorf("client read failed: %v", err)
		}

		var decodedRequest map[string]interface{}
		decoder := plist.NewDecoder(bytes.NewReader(request))
		err = decoder.Decode(&decodedRequest)
		if err != nil {
			t.Close()
			t.logger.Errorln("failed decoding", request, err)
		}

		t.logger.Infof("read UsbMuxMessage:%v", decodedRequest)

		err = lockdownToDevice.Send(decodedRequest)
		if err != nil {
			t.logger.Errorf("Failed forwarding message to device: %x", request)
		}

		response, err := lockdownToDevice.ReadMessage()
		if err != nil {
			t.logger.Errorf("error reading from device: %+v", err)
			panic(err)
			response, err = lockdownToDevice.ReadMessage() // FIXME
			t.logger.Infof("second read: %+v %+v", response, err)
		}

		var decodedResponse map[string]interface{}
		decoder = plist.NewDecoder(bytes.NewReader(response))
		err = decoder.Decode(&decodedResponse)
		if err != nil {
			t.logger.Errorln("Failed decoding LockdownMessage", decodedResponse, err)
		}

		if decodedResponse["EnableSessionSSL"] == true {
			panic("EnableSessionSSL==true")
		}

		if decodedResponse["Request"] == "StartService" && decodedResponse["Error"] == nil {
			useSSL := false
			if decodedResponse["EnableServiceSSL"] != nil {
				useSSL = decodedResponse["EnableServiceSSL"].(bool)
			}
			info := debugproxy.PhoneServiceInformation{
				ServicePort: uint16(decodedResponse["Port"].(uint64)),
				ServiceName: decodedResponse["Service"].(string),
				UseSSL:      useSSL}

			t.logger.Debugf("Detected Service Start:%+v", info)
		}

		if decodedResponse["Request"] == "StopSession" {
			t.logger.Info("Stop Session detected, disabling SSL")
			// lockdownOnUnixSocket.DisableSessionSSL()
			lockdownToDevice.DisableSessionSSL()
		}
	}
}
