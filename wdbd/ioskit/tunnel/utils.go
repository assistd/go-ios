package tunnel

import (
	"errors"

	"github.com/danielpaulus/go-ios/ios"
)

func GetDevice(udid string) (ios.DeviceEntry, error) {
	conn, err := ios.NewUsbMuxConnectionSimple()
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	defer conn.Close()
	lists, err := conn.ListDevices()
	if err != nil {
		return ios.DeviceEntry{}, err
	}

	for _, l := range lists.DeviceList {
		if l.Properties.SerialNumber == udid {
			return l, nil
		}
	}

	return ios.DeviceEntry{}, errors.New("not found")
}
