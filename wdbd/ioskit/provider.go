package ioskit

import (
	"fmt"
	"net"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type Provider struct {
	socket     string
	device     *RemoteDevice // serial -> RemoteDevice
	deviceID   int
	services   []*PhoneService
	counter    int
	pairRecord ios.PairRecord
	mutex      sync.Mutex
}

// NewUsbmuxd create an Usbmuxd instance
func NewProvider(socket string, device *RemoteDevice) (*Provider, error) {
	pair, err := device.ReadPairRecord()
	if err != nil {
		return nil, fmt.Errorf("read pair record failed:%v", err)
	}

	entry, err := device.ListDevices()
	if err != nil {
		return nil, fmt.Errorf("read device:%v", err)
	}

	return &Provider{
		socket:     socket,
		device:     device,
		deviceID:   entry.DeviceID,
		pairRecord: pair,
	}, nil
}

func makeMuxConn(conn net.Conn) *ios.UsbMuxConnection {
	deviceConn := ios.NewDeviceConnectionWithConn(conn)
	return ios.NewUsbMuxConnection(deviceConn)
}

func (p *Provider) spawnService(serviceInfo *PhoneService) {
	p.mutex.Lock()
	p.services = append(p.services, serviceInfo)
	p.mutex.Unlock()
	go serviceInfo.Proxy(p)
}

func (p *Provider) connectToDevice(deviceID int) (net.Conn, *ios.LockDownConnection, error) {
	netConn, err := p.device.NewConn(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to device's usbmuxd failed:%v", err)
	}

	deviceConn := ios.NewDeviceConnectionWithConn(netConn)
	usbmuxConn := ios.NewUsbMuxConnection(deviceConn)
	lockdownToDevice, err := usbmuxConn.ConnectLockdown(deviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to lockdown failed: %v", err)
	}

	return netConn, lockdownToDevice, nil
}

func (p *Provider) savePairFromRemote() error {
	muxConn, err := ios.NewUsbMuxConnectionSimple()
	if err != nil {
		return err
	}
	defer muxConn.Close()

	pair := p.pairRecord
	// 两个要点
	// 1. Usbmuxd会使用DeviceCertificate与真正的设备使用ssl握手，由于我们没有设备的私钥，只有证书（证书中含有公钥），
	//    这里直接把设备的私钥替换为设备所连PC的证书（内含公钥），我们的程序使用PC的私钥即可与usbmuxd ssl握手成功。
	// 2. 注册用的wifi地址必须与pairRecord的WiFiMACAddress一致
	pair.DeviceCertificate = pair.HostCertificate

	udid := p.device.Serial
	pairRecordData := ios.SavePair{
		BundleID:            "go.ios.control",
		ClientVersionString: "go-ios-1.0.0",
		MessageType:         "SavePairRecord",
		ProgName:            "go-ios",
		LibUSBMuxVersion:    3,
		PairRecordID:        udid,
		PairRecordData:      ios.ToPlistBytes(pair),
	}
	err = muxConn.Send(pairRecordData)
	return err
}

// Run serve a tcp server, and do the message switching between remote usbmuxd with local one
func (p *Provider) Run() error {
	listener, err := net.Listen("tcp", p.socket)
	if err != nil {
		return err
	}

	err = p.savePairFromRemote()
	if err != nil {
		panic(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("error with connection: %e", err)
			// FIXME: close all clients
			return err
		}

		lockdownFromClient := ios.NewLockDownConnection(ios.NewDeviceConnectionWithConn(conn))
		t := NewLockDownTransport(p, lockdownFromClient, p.pairRecord, p.device)

		go func() {
			t.Proxy()
			t.Close()
			lockdownFromClient.Close()
		}()
	}
}
