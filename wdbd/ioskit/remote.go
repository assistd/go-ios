package ioskit

import (
	"context"
	"fmt"
	"net"

	"github.com/danielpaulus/go-ios/ios"
)

var (
	muxd *RemoteDevice
)

type RemoteDevice struct {
	Addr     string
	Serial   string // udid
	registry *Registry
}

func NewRemoteDevice(addr, serial string) *RemoteDevice {
	return &RemoteDevice{
		Addr:     addr,
		Serial:   serial,
		registry: NewRegistry(),
	}
}

func (r *RemoteDevice) Monitor(ctx context.Context) error {
	kit, _ := NewDeviceMonitor("tcp", r.Addr)
	err := kit.Monitor(ctx, r.registry, -1)
	return err
}

func (r *RemoteDevice) ListDevices() (DeviceEntry, error) {
	return r.registry.DeviceBySerial(r.Serial)
}

func (r *RemoteDevice) ReadPairRecord() (ios.PairRecord, error) {
	conn, err := r.NewConn(nil)
	if err != nil {
		return ios.PairRecord{}, err
	}
	defer conn.Close()

	devConn := ios.NewDeviceConnectionWithConn(conn)
	muxConn := ios.NewUsbMuxConnection(devConn)
	return muxConn.ReadPair(r.Serial)
}

func (r *RemoteDevice) NewConn(ctx context.Context) (net.Conn, error) {
	conn, err := net.Dial("tcp", r.Addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (r *RemoteDevice) ConnectLockdown() (*ios.UsbMuxConnection, *ios.LockDownConnection, error) {
	device, err := r.ListDevices()
	if err != nil {
		return nil, nil, err
	}

	netConn, err := r.NewConn(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to device's usbmuxd failed:%v", err)
	}

	deviceConn := ios.NewDeviceConnectionWithConn(netConn)
	usbmuxConn := ios.NewUsbMuxConnection(deviceConn)
	lockdownToDevice, err := usbmuxConn.ConnectLockdown(device.DeviceID)
	if err != nil {
		netConn.Close()
		return nil, nil, fmt.Errorf("connect to lockdown failed: %v", err)
	}

	return usbmuxConn, lockdownToDevice, nil
}

func (r *RemoteDevice) ConnectLockdownWithSession() (*ios.LockDownConnection, error) {
	device, err := r.ListDevices()
	if err != nil {
		return nil, err
	}

	conn, err := r.NewConn(nil)
	if err != nil {
		return nil, fmt.Errorf("connect to device's usbmuxd failed:%v", err)
	}

	deviceConn := ios.NewDeviceConnectionWithConn(conn)
	usbmuxConn := ios.NewUsbMuxConnection(deviceConn)
	pairRecord, err := usbmuxConn.ReadPair(r.Serial)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not retrieve PairRecord with error: %v", err)
	}

	lockdownConnection, err := usbmuxConn.ConnectLockdown(device.DeviceID)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect to lockdown failed: %v", err)
	}

	resp, err := lockdownConnection.StartSession(pairRecord)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("StartSession failed: %+v error: %v", resp, err)
	}
	return lockdownConnection, nil
}

func (r *RemoteDevice) GetInfo() (ios.AllValuesType, error) {
	lockdownConnection, err := r.ConnectLockdownWithSession()
	if err != nil {
		return ios.AllValuesType{}, fmt.Errorf("connect to lockdown failed: %v", err)
	}
	defer lockdownConnection.Close()
	allValues, err := lockdownConnection.GetValues()
	if err != nil {
		return ios.AllValuesType{}, err
	}

	return allValues.Value, nil
}
