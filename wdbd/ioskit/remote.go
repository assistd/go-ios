package ioskit

import (
	"context"
	"net"

	"github.com/danielpaulus/go-ios/ios"
)

var (
	muxd *RemoteDevice
)

type RemoteDevice struct {
	Addr     string
	Serial   string
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
