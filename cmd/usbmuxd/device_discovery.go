package main

import "fmt"

var remoteManager *RemoteDeviceManager

type RemoteDevice struct {
	Host string
	Port uint16
}

type RemoteDeviceManager struct {
	lists map[string]RemoteDevice
}

func (r *RemoteDeviceManager) Add(device RemoteDevice) {
	key := fmt.Sprintf("%s:%d", device.Host, device.Port)
	r.lists[key] = device
}

func (r *RemoteDeviceManager) Remove(device RemoteDevice) {
	key := fmt.Sprintf("%s:%d", device.Host, device.Port)
	if _, ok := r.lists[key]; ok {
		delete(r.lists, key)
	}
}

func NewRemoteManager() *RemoteDeviceManager {
	return &RemoteDeviceManager{
		lists: make(map[string]RemoteDevice),
	}
}

func InitRemoteDeviceManager() {
	remoteManager = NewRemoteManager()
	remoteManager.Add(RemoteDevice{
		Host: "127.0.0.1",
		Port: 60000,
	})
}
