package main

import (
	"os"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
	log "github.com/sirupsen/logrus"
)

func TestDeviceInfo() {
	device, err := ios.GetDevice("")
	if err != nil {
		log.Fatal(err)
	}

	dvt, err := dvt.NewDvtSecureSocketProxyService(device)
	if err != nil {
		log.Fatal(err)
	}
	defer dvt.Close()

	deviceInfo, err := instruments.NewDeviceInfo(dvt)
	if err != nil {
		log.Fatal(err)
	}

	deviceInfo.Proclist()
	os.Exit(0)
}
