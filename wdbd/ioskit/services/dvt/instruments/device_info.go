package instruments

import (
	"fmt"

	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services/dvt"
	log "github.com/sirupsen/logrus"
)

type DeviceInfo struct {
	channel services.Channel
}

func NewDeviceInfo(dvt *dvt.DvtSecureSocketProxyService) (*DeviceInfo, error) {
	const identifier = "com.apple.instruments.server.services.deviceinfo"
	log.Infoln("deviceinfo: MakeChannel")
	channel, err := dvt.MakeChannel(identifier)
	if err != nil {
		log.Infoln("deviceinfo: ", err)
		return nil, err
	}

	log.Infoln("deviceinfo: ", channel)
	s := &DeviceInfo{channel}
	return s, nil
}

// List a directory.
func (d *DeviceInfo) Proclist() {
	log.Infoln("deviceinfo: runningProcesses")
	f, err := d.channel.Call("runningProcesses")
	if err != nil {
		panic(err)
	}

	data, _, err := f.Parse()
	// log.Infof("proclist: sel=%v, aux=%#v, exWrr=%v", data, aux, err)
	if err != nil {
		panic(err)
	}

	procList, ok := data[0].([]interface{})
	if !ok {
		panic(err)
	}

	for i, procMap := range procList {
		fmt.Printf("[%v] %#v\n", i, procMap)
	}
}
