package ioskit

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"runtime"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

var lockdownId int32

type LockDownTransport struct {
	*ios.LockDownConnection
	provider   *Provider
	pairRecord ios.PairRecord
	id         int32
	device     *RemoteDevice
	logger     *log.Entry
}

func NewLockDownTransport(provider *Provider, conn *ios.LockDownConnection, pairRecord ios.PairRecord, device *RemoteDevice) *LockDownTransport {
	lockdownId++
	return &LockDownTransport{
		LockDownConnection: conn,
		provider:           provider,
		pairRecord:         pairRecord,
		id:                 lockdownId,
		device:             device,
		logger:             log.WithField("id", lockdownId),
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

const wirelessLockdown = "com.apple.mobile.wireless_lockdown"
const developerdomain = "com.apple.xcode.developerdomain"

func (t *LockDownTransport) filter(decodedRequest map[string]interface{}) (map[string]interface{}, bool) {
	if runtime.GOOS != "darwin" {
		return nil, false
	}

	// defaults delete com.apple.dt.Xcode DVTDeviceTokens
	// defaults delete com.apple.dt.xcodebuild DVTDeviceTokens
	// defaults delete com.apple.iTunes WirelessBuddyID
	// 使得xcode中可以看到设备，这里的两个Value是从某台iPhoneX中抓取协议获取的，也许随便什么数值都行。
	// 讨论细节：https://github.com/assistd/go-ios/issues/44#issuecomment-1387269808
	if decodedRequest["Domain"] == wirelessLockdown && decodedRequest["Request"] == "GetValue" {
		if decodedRequest["Key"] == "EnableWifiDebugging" {
			return map[string]interface{}{"Domain": wirelessLockdown, "Key": "EnableWifiDebugging", "Request": "GetValue", "Value": true}, true
		} else if decodedRequest["Key"] == "WirelessBuddyID" {
			return map[string]interface{}{"Domain": wirelessLockdown, "Key": "WirelessBuddyID", "Request": "GetValue", "Value": t.provider.xcode.wirelessBuddyID}, true
		} else if decodedRequest["Key"] == "EnableWifiConnections" {
			return map[string]interface{}{"Domain": wirelessLockdown, "Key": "EnableWifiConnections", "Request": "GetValue", "Value": true}, true
		}
	} else if decodedRequest["Domain"] == developerdomain && decodedRequest["Request"] == "GetValue" {
		if decodedRequest["Key"] == "WirelessHosts" {
			return map[string]interface{}{"Domain": developerdomain, "Key": "WirelessHosts", "Request": "GetValue", "Value": []string{t.provider.xcode.wirelessHosts}}, true
		}
	}

	return nil, false
}

func (t *LockDownTransport) Proxy() error {
	_, lockdownToDevice, err := t.connectToDevice()
	if err != nil {
		return nil
	}
	defer lockdownToDevice.Close()
	defer t.Close()

	for {
		request, err := t.ReadMessage()
		if err != nil {
			if len(request) > 0 {
				t.logger.Errorln(hex.Dump(request))
			}
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

		t.logger.Infof("--> UsbMuxMessage:%v", decodedRequest)
		if decodedRequest["Request"] == "StartSession" {
			decodedRequest["HostID"] = t.pairRecord.HostID
			decodedRequest["SystemBUID"] = t.pairRecord.SystemBUID
		}

		err = lockdownToDevice.Send(decodedRequest)
		if err != nil {
			t.logger.Errorf("Failed forwarding message to device: %x", request)
		}

		response, err := lockdownToDevice.ReadMessage()
		if err != nil {
			t.logger.Errorf("error reading from device: %+v", err)
			panic(err)
		}

		var decodedResponse map[string]interface{}
		decoder = plist.NewDecoder(bytes.NewReader(response))
		err = decoder.Decode(&decodedResponse)
		if err != nil {
			t.logger.Errorln("Failed decoding LockdownMessage", decodedResponse, err)
		} else {
			t.logger.Infoln("<-- response", decodedResponse)
		}

		resp, ok := t.filter(decodedRequest)
		if ok {
			decodedResponse = resp
			t.logger.Infoln("replace response to ", resp)
		}

		err = t.Send(decodedResponse)
		if err != nil {
			t.logger.Warningln("Failed sending LockdownMessage from device to host service", decodedResponse, err)
		}

		if decodedResponse["EnableSessionSSL"] == true {
			if err := lockdownToDevice.EnableSessionSsl(t.pairRecord); err != nil {
				t.logger.Errorln("enable ssl failed:", err)
				return err
			}
			if err := t.EnableSessionSslServerMode(t.pairRecord); err != nil {
				t.logger.Errorln("enable ssl server failed:", err)
				return err
			}
		}

		if decodedResponse["Request"] == "StartService" && decodedResponse["Error"] == nil {
			useSSL := false
			if decodedResponse["EnableServiceSSL"] != nil {
				useSSL = decodedResponse["EnableServiceSSL"].(bool)
			}
			info := &PhoneService{
				Port:   uint16(decodedResponse["Port"].(uint64)),
				Name:   decodedResponse["Service"].(string),
				UseSSL: useSSL}
			t.provider.spawnService(info)
			t.logger.Infof("Detected Service Start:%+v", info)
		}

		if decodedResponse["Request"] == "StopSession" {
			t.logger.Info("Stop Session detected, disabling SSL")
			// lockdownOnUnixSocket.DisableSessionSSL()
			lockdownToDevice.DisableSessionSSL()
		}
	}
}
