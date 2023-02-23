package tunnel

import (
	"errors"
	"fmt"

	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
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

func ToMap(msg *dtx.Message) (string, map[string]interface{}, error) {
	if len(msg.Payload) != 1 {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v has payload size !=1", msg)
	}
	selector, ok := msg.Payload[0].(string)
	if !ok {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v payload: %+v wasn't a string", msg, msg.Payload[0])
	}
	args := msg.Auxiliary.GetArguments()
	if len(args) == 0 {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v has an empty auxiliary dictionary", msg)
	}

	data, ok := args[0].([]byte)
	if !ok {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v invalid aux", msg)
	}

	unarchived, err := nskeyedarchiver.Unarchive(data)
	if err != nil {
		return "", map[string]interface{}{}, err
	}
	if len(unarchived) == 0 {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v invalid aux", msg)
	}

	aux, ok := unarchived[0].(map[string]interface{})
	if !ok {
		return "", map[string]interface{}{}, fmt.Errorf("error extracting, msg %+v auxiliary: %+v didn't contain a map[string]interface{}", msg, msg.Payload[0])
	}

	return selector, aux, nil
}

func ExtractMapPayload(message *dtx.Message) (map[string]interface{}, error) {
	if len(message.Payload) != 1 {
		return map[string]interface{}{}, fmt.Errorf("payload of message should have only one element: %+v", message)
	}
	response, ok := message.Payload[0].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, fmt.Errorf("payload type of message should be map[string]interface{}: %+v", message)
	}
	return response, nil
}
