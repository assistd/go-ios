package ioskit

import (
	"fmt"
	"net"
	"runtime"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/mackit"
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
	xcode      *XcodeDebugging
}

type XcodeDebugging struct {
	wirelessHosts   string
	wirelessBuddyID string
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
	go func() {
		var err error
		switch serviceInfo.Name {
		case "com.apple.mobile.assertion_agent":
			s := PowerAssertionService(*serviceInfo)
			err = s.Proxy(p)
		default:
			err = serviceInfo.Proxy(p)
		}
		log.Errorf("service proxy: %v end: %v", serviceInfo, err)
	}()
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
	// 两个要点
	// 1. Usbmuxd会使用DeviceCertificate与真正的设备使用ssl握手，由于我们没有设备的私钥，只有证书（证书中含有公钥），
	//    这里直接把设备的私钥替换为设备所连PC的证书（内含公钥），我们的程序使用PC的私钥即可与usbmuxd ssl握手成功。
	// 2. 注册用的wifi地址必须与pairRecord的WiFiMACAddress一致
	pair.DeviceCertificate = pair.HostCertificate
	pair.SystemBUID = buid

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

func BuildDeviceInfo(values ios.AllValuesType) mackit.DeviceInfo {
	entry := mackit.DeviceInfo{}
	entry.UUID = values.UniqueDeviceID
	entry.Serial = values.SerialNumber
	entry.BuildVersion = values.BuildVersion
	entry.WiFiAddress = values.WiFiAddress
	entry.DeviceType = values.ProductType
	entry.DeviceName = values.ProductName
	entry.ProductVersion = values.ProductVersion
	entry.HardwareModel = values.HardwareModel
	entry.DeviceArchitecture = values.CPUArchitecture
	entry.DeviceBluetoothMAC = values.BluetoothAddress
	entry.ChipID = values.ChipID
	return entry
}

func (p *Provider) EnableXcode() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("only support on macOS")
	}

	// defaults read com.apple.dt.Xcode DVTDeviceTokens
	udid := p.device.Serial
	if ok := mackit.FindDevice(udid); !ok {
		info, err := p.device.GetInfo()
		if err != nil {
			log.Errorln("get info failed:", err)
			return err
		}
		err = mackit.AddDevice(BuildDeviceInfo(info))
		if err != nil {
			log.Errorln("add device failed:", err)
			return err
		}
	}

	// defaults read com.apple.iTunes WirelessBuddyID
	wirelessBuddyID, err := mackit.ReadWirelessBuddyID()
	if err != nil {
		wirelessBuddyID = mackit.AllocateWriteWirelessID()
		err = mackit.WriteWirelessBuddyID(wirelessBuddyID)
		if err != nil {
			log.Errorln("write WirelessBuddyID failed:", err)
			return err
		}
	}

	wirelessHosts, err := mackit.GetUdid()
	if err != nil {
		log.Errorln("write wirelessHosts failed:", err)
		return err
	}

	p.xcode = &XcodeDebugging{
		wirelessHosts:   wirelessHosts,
		wirelessBuddyID: wirelessBuddyID,
	}
	log.Infoln("XcodeDebugging:", p.xcode)

	return nil
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

	err = p.EnableXcode()
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
