package main

import (
	"os"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/xctest"
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

	pid, err := deviceInfo.Launch("com.teapotapps.iperf", nil, nil, false, false)
	log.Infoln(pid, err)
}

func TestXctest(tms, tms2 *dvt.TestManagerdSecureService, sps *dvt.DvtSecureSocketProxyService) {
	runner, err := xctest.NewXctestRunner(tms, tms2, sps)
	if err != nil {
		log.Fatal(err)
	}

	info := xctest.XctestAppInfo{
		BundleID:             "com.wetest.wda-scrcpy.xctrunner",
		TestRunnerBundleID:   "com.wetest.wda-scrcpy.xctrunner",
		XctestConfigFileName: "scrcpy.xctest",
	}
	err = runner.Xctest(info, nil, nil, false)
	log.Errorln(err)
}

func TestDvt() {
	device, err := ios.GetDevice("")
	if err != nil {
		log.Fatal(err)
	}

	// TestLaunch(sps)
	// os.Exit(0)

	tms, err := dvt.NewTestManagerdSecureService(device)
	if err != nil {
		log.Fatal(err)
	}
	defer tms.Close()

	tms2, err := dvt.NewTestManagerdSecureService(device)
	if err != nil {
		log.Fatal(err)
	}
	defer tms2.Close()

	sps, err := dvt.NewDvtSecureSocketProxyService(device)
	if err != nil {
		log.Fatal(err)
	}
	defer sps.Close()

	TestXctest(tms, tms2, sps)
	os.Exit(0)
}
