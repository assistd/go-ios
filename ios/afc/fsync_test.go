package afc

import (
	"fmt"
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

func TestConnection_listDir(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	flist, err := conn.listDir("/DCIM/")
	if err != nil {
		log.Fatalf("tree view failed:%v", err)
	}
	for _, v := range flist {
		fmt.Printf("path: %+v\n", v)
	}
}

func TestConnection_TreeView(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	err = conn.TreeView("/DCIM/", "", true)
	if err != nil {
		log.Fatalf("tree view failed:%v", err)
	}
}

func TestConnection_pullSingleFile(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		log.Fatalf("connect service failed: %v", err)
	}

	err = conn.pullSingleFile("/DCIM/architecture_diagram.png", "architecture_diagram.png")
	if err != nil {
		log.Fatalf("tree view failed:%v", err)
	}
}