package afc

import (
	"github.com/danielpaulus/go-ios/ios"
	"log"
	"testing"
)

const test_device_udid = "f90589e357ef231602d3bbed14ba748af2ed8373"

func TestConnection_Remove(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	err = conn.Remove("/DCIM/goios")
	if err != nil {
		log.Fatalf("remove failed:%v", err)
	}
}

func TestConnection_Mkdir(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	err = conn.MakeDir("/DCIM/TestDir")
	if err != nil {
		log.Fatalf("remove failed:%v", err)
	}
}

func TestConnection_stat(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	si, err := conn.stat("/DCIM/architecture_diagram.png")
	if err != nil {
		log.Fatalf("get stat failed:%v", err)
	}
	log.Printf("stat :%+v", si)
}
