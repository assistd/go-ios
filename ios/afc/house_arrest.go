package afc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"howett.net/plist"
)

const houseArrestServiceName = "com.apple.mobile.house_arrest"

const (
	VendContainer = "VendContainer"
	VendDocuments = "VendDocuments"
)

type vendContainerResponse struct {
	Status string
	Error  string
}

func NewHouseArrestContainerFs(device ios.DeviceEntry, bundleId string) (*Fsync, error) {
	conn, err := NewHouseArrestConn(device, bundleId, VendContainer)
	if err != nil {
		return nil, err
	}
	return &Fsync{conn, HouseArrestContainerFs, bundleId}, nil
}

func NewHouseArrestDocumentFs(device ios.DeviceEntry, bundleId string) (*Fsync, error) {
	conn, err := NewHouseArrestConn(device, bundleId, VendDocuments)
	if err != nil {
		return nil, err
	}
	return &Fsync{conn, HouseArrestDocumentFs, bundleId}, nil
}

func NewHouseArrestConn(device ios.DeviceEntry, bundleId string, containerType string) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, houseArrestServiceName)
	if err != nil {
		return nil, err
	}
	err = vendContainer(deviceConn, bundleId, containerType)
	if err != nil {
		return nil, err
	}
	return &Connection{deviceConn: deviceConn}, nil
}

func vendContainer(deviceConn ios.DeviceConnectionInterface, bundleID string, containerType string) error {
	plistCodec := ios.NewPlistCodec()
	vendContainer := map[string]interface{}{"Command": containerType, "Identifier": bundleID}
	msg, err := plistCodec.Encode(vendContainer)
	if err != nil {
		return fmt.Errorf("VendContainer Encoding cannot fail unless the encoder is broken: %v", err)
	}
	err = deviceConn.Send(msg)
	if err != nil {
		return err
	}
	reader := deviceConn.Reader()
	response, err := plistCodec.Decode(reader)
	if err != nil {
		return err
	}
	return checkResponse(response)
}

func checkResponse(vendContainerResponseBytes []byte) error {
	response, err := plistFromBytes(vendContainerResponseBytes)
	if err != nil {
		return err
	}
	if "Complete" == response.Status {
		return nil
	}
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return errors.New("unknown error during vendcontainer")
}

func plistFromBytes(plistBytes []byte) (vendContainerResponse, error) {
	var vendResponse vendContainerResponse
	decoder := plist.NewDecoder(bytes.NewReader(plistBytes))

	err := decoder.Decode(&vendResponse)
	if err != nil {
		return vendResponse, err
	}
	return vendResponse, nil
}
