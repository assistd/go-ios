package main

import (
	"fmt"

	"github.com/grandcat/zeroconf"
)

type MdnsProxy struct {
	Name    string
	service string
	domain  string
	port    int
	Host    string
	Ip      string
	server  *zeroconf.Server
}

func NewMdnsProxy(wifiAddress, udid, ip string) *MdnsProxy {
	// @字符实际应为手机的ipv6地址，由于ipv6没有得到广泛支持，这里随意取值即可
	name := fmt.Sprintf("%s@fe80::72ea:5aff:fe2a:88ad", wifiAddress)
	return &MdnsProxy{
		Name:    name,
		service: "_apple-mobdev._tcp.",
		domain:  "local.",
		Host:    udid,
		port:    32498,
		Ip:      ip,
	}
}

func (m *MdnsProxy) Register() error {
	server, err := zeroconf.RegisterProxy(m.Name, m.service, m.domain, m.port, m.Host, []string{m.Ip}, nil, nil)
	if err != nil {
		return fmt.Errorf("register mdns proxy failed: %v", err)
	}

	m.server = server
	return nil
}

func (m *MdnsProxy) Shutdown() {
	if m.server != nil {
		m.server.Shutdown()
	}
}
