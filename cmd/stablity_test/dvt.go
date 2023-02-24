package main

import (
	"os"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
	log "github.com/sirupsen/logrus"
)

func TestDeviceInfo(dvt *dvt.DvtSecureSocketProxyService) {
	deviceInfo, err := instruments.NewDeviceInfo(dvt)
	if err != nil {
		log.Fatal(err)
	}

	deviceInfo.Proclist()
}

func TestLaunch(dvt *dvt.DvtSecureSocketProxyService) {
	deviceInfo, err := instruments.NewProcessControl(dvt)
	if err != nil {
		log.Fatal(err)
	}

	pid, err := deviceInfo.Launch("com.example.multiTouch", nil, nil, false, false)
	log.Infoln(pid, err)
}

func TestDvt() {
	device, err := ios.GetDevice("")
	if err != nil {
		log.Fatal(err)
	}

	dvt, err := dvt.NewDvtSecureSocketProxyService(device)
	if err != nil {
		log.Fatal(err)
	}
	defer dvt.Close()

	TestLaunch(dvt)
	os.Exit(0)
}
