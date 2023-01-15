package ioskit

import (
	"bytes"
	"fmt"
	"net"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/debugproxy"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

var lockdownId int32

type LockDownTransport struct {
	*ios.LockDownConnection
	id     int32
	device *RemoteDevice
	logger *log.Entry
}

func NewLockDownTransport(conn *ios.LockDownConnection, device *RemoteDevice) *LockDownTransport {
	lockdownId++
	return &LockDownTransport{
		conn,
		lockdownId,
		device,
		log.WithField("id", lockdownId),
	}
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

func (t *LockDownTransport) Proxy() error {
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
		} else {
			t.logger.Infoln("<-- response", decodedResponse)
		}

		err = t.Send(decodedResponse)
		if err != nil {
			t.logger.Warningln("--> Failed sending LockdownMessage from device to host service", decodedResponse, err)
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
