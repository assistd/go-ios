package ioskit

import (
	"fmt"
	"net"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/debugproxy"
	log "github.com/sirupsen/logrus"
)

type Provider struct {
	socket     string
	device     *RemoteDevice // serial -> RemoteDevice
	services   []debugproxy.PhoneServiceInformation
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

	return &Provider{
		socket:     socket,
		device:     device,
		pairRecord: pair,
	}, nil
}

func makeMuxConn(conn net.Conn) *ios.UsbMuxConnection {
	deviceConn := ios.NewDeviceConnectionWithConn(conn)
	return ios.NewUsbMuxConnection(deviceConn)
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

	buid, err := muxConn.ReadBuid()
	if err != nil {
		return err
	}

	pair := p.pairRecord
	pair.SystemBUID = buid
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
		}

		lockdownFromClient := ios.NewLockDownConnection(ios.NewDeviceConnectionWithConn(conn))
		t := NewLockDownTransport(lockdownFromClient, p.pairRecord, p.device)

		go func() {
			t.Proxy()
			t.Close()
			lockdownFromClient.Close()
		}()
	}
}
