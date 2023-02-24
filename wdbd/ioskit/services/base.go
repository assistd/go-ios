package services

import (
	"errors"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type BaseService struct {
	Name        string
	IsDeveloper bool
	Conn        ios.DeviceConnectionInterface
	codec       ios.PlistCodec
	udid        string
	deviceID    int
	inited      bool
}

func (b *BaseService) init(device ios.DeviceEntry) error {
	if b.inited {
		return errors.New("already inited")
	}

	deviceConn, err := ios.ConnectToService(device, b.Name)
	if err != nil {
		return err
	}

	b.codec = ios.NewPlistCodec()
	b.Conn = deviceConn
	b.udid = device.Properties.SerialNumber
	b.deviceID = device.DeviceID
	b.inited = true
	return nil
}

func (b *BaseService) GetDevice() ios.DeviceEntry {
	return ios.DeviceEntry{
		DeviceID: b.deviceID,
		Properties: ios.DeviceProperties{
			SerialNumber: b.udid,
		},
	}
}

func (b *BaseService) Send(data interface{}) error {
	bytes, err := b.codec.Encode(data)
	if err != nil {
		return err
	}

	err = b.Conn.Send(bytes)
	if err != nil {
		return err
	}
	return err
}

func (b *BaseService) Receive() (map[string]interface{}, error) {
	bytes, err := b.codec.Decode(b.Conn.Reader())
	if err != nil {
		return nil, err
	}

	resp, err := ios.ParsePlist(bytes)
	log.Infof("%#v", resp)
	return resp, err
}

func (b *BaseService) SendReceive(data interface{}) (map[string]interface{}, error) {
	bytes, err := b.codec.Encode(data)
	if err != nil {
		return nil, err
	}

	err = b.Conn.Send(bytes)
	if err != nil {
		return nil, err
	}

	bytes, err = b.codec.Decode(b.Conn.Reader())
	if err != nil {
		return nil, err
	}

	resp, err := ios.ParsePlist(bytes)
	log.Infof("%#v", resp)

	return resp, err
}

func (b *BaseService) Close() {
	if b.inited && b.Conn != nil {
		b.Conn.Close()
	}
}
