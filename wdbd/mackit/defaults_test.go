package mackit_test

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/mackit"
)

func TestFindDevice(t *testing.T) {
	ok := mackit.FindDevice("fe32ecec58d608c8735f7f8ca67ca99bdea10ee3")
	t.Logf("find device:%v", ok)

	ok = mackit.FindDevice("00008110-00142C9A3642801E")
	t.Logf("find device:%v", ok)
}

func TestAddDevice(t *testing.T) {
	udid := "fe32ecec58d608c8735f7f8ca67ca99bdea10ee3"
	device, err := ios.GetDevice(udid)
	if err != nil {
		t.Fatal(err)
	}
	allValues, err := ios.GetValuesPlist(device)
	if err != nil {
		t.Fatal(err)
	}

	info := mackit.BuildDeviceInfo(allValues)
	err = mackit.AddDevice(info)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadWirelessBuddyID(t *testing.T) {
	uuid, err := mackit.ReadWirelessBuddyID()
	t.Logf("uuid=%v, err:=%v", uuid, err)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteWirelessBuddyID2(t *testing.T) {
	uuid, err := mackit.WriteWirelessBuddyID2()
	t.Logf("uuid=%v, err:=%v", uuid, err)
	if err != nil {
		t.Fatal(err)
	}
}
