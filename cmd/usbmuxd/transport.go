package main

import (
	"io"
	"net"
	"sync"
)

type Transport struct {
	Serial      string
	remoteConn  *net.TCPConn
	selfLocalId uint32
	connMap     map[uint32]io.ReadWriteCloser
	mutex       sync.Mutex
}

// NewTransport init transport
func NewTransport(conn *net.TCPConn, serial string) *Transport {
	return &Transport{
		Serial:     serial,
		remoteConn: conn,
		connMap:    make(map[uint32]io.ReadWriteCloser),
	}
}

// Kick kick off the remote adb server's connection
func (t *Transport) Kick() {
	_ = t.remoteConn.Close()
}

// HandleLoop run adb packet reading and writing loop
func (t *Transport) HandleLoop() {
	//ctx, cancel := context.WithCancel(context.Background())
}
