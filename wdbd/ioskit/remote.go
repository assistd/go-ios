package ioskit

import (
	"context"
)

type RemoteDevice struct {
	Addr        string
	Serial      string
	iosDeviceId int
}

func NewRemoteDevice(ctx context.Context, addr, serial string) *RemoteDevice {
	return &RemoteDevice{
		Addr:   addr,
		Serial: serial,
	}
}
