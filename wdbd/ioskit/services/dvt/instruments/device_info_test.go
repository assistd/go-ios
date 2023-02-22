package instruments_test

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt/instruments"
)

func TestDeviceInfo(t *testing.T) {
	device, err := ios.GetDevice("")
	if err != nil {
		t.Fatal(err)
	}

	dvt, err := dvt.NewDvtSecureSocketProxyService(device)
	if err != nil {
		t.Fatal(err)
	}
	defer dvt.Close()

	deviceInfo, err := instruments.NewDeviceInfo(dvt)
	if err != nil {
		t.Fatal(err)
	}

	deviceInfo.Proclist()
}
