package services

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
)

type MobileImageMounterService struct {
	BaseService
}

var (
	// ErrUnsupported given command isn't supported for this iOS version
	ErrUnsupported               = errors.New("unsupportedCommandError")
	ErrInternal                  = errors.New("some internal Apple error")
	ErrNotMounted                = errors.New("given image for umount wasn't mounted in the first place")
	ErrMessageNotSupported       = errors.New("messageNotSupportedError")
	ErrDeveloperModeIsNotEnabled = errors.New("developerModeIsNotEnabledError")
)

func NewMobileImageMounterService(device ios.DeviceEntry) (*MobileImageMounterService, error) {
	s := &MobileImageMounterService{
		BaseService: BaseService{
			Name:        "com.apple.mobile.mobile_image_mounter",
			IsDeveloper: false,
		},
	}
	err := s.init(device)
	return s, err
}

func (s *MobileImageMounterService) Init() error {
	return nil
}

func (s *MobileImageMounterService) QueryDeveloperModeStatus() (bool, error) {
	req := map[string]interface{}{
		"Command": "QueryDeveloperModeStatus",
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return false, err
	}

	status, ok := resp["DeveloperModeStatus"]
	if !ok {
		return false, ErrMessageNotSupported
	}

	return status.(bool), nil
}

// CopyDevices copy mounted devices list
func (s *MobileImageMounterService) CopyDevices() (map[string]interface{}, error) {
	req := map[string]interface{}{
		"Command": "CopyDevices",
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return nil, err
	}

	entryList, ok := resp["EntryList"]
	if !ok {
		return nil, ErrMessageNotSupported
	}

	array, _ := entryList.(map[string]interface{})
	return array, nil
}

// LookupImage Lookup mounted 'Developer' image
func (s *MobileImageMounterService) LookupImage() (map[string]interface{}, error) {
	req := map[string]interface{}{
		"Command":   "LookupImage",
		"ImageType": "Developer",
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return nil, err
	}

	if _, ok := resp["Error"]; ok {
		return nil, fmt.Errorf("mount error:%#v", resp)
	}

	signatures, ok := resp["ImageSignature"]
	if !ok {
		return nil, fmt.Errorf("invalid response: %#v", resp)
	}

	array, _ := signatures.(map[string]interface{})
	return array, nil
}

func (s *MobileImageMounterService) Mount(imagePath, signaturePath string) error {
	_, err := s.LookupImage()
	if err != nil {
		return nil
	}

	err = s.uploadImage(imagePath, signaturePath)
	if err != nil {
		return err
	}
	signature, err := os.ReadFile(signaturePath)
	if err != nil {
		return err
	}

	req := map[string]interface{}{
		"Command":        "MountImage",
		"ImageType":      "Developer",
		"ImageSignature": signature,
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return err
	}

	desc, ok := resp["DetailedError"]
	if ok {
		if strings.Contains(desc.(string), "Developer mode is not enabled") {
			return ErrDeveloperModeIsNotEnabled
		}
	}
	if resp["Status"] != "Complete" {
		return fmt.Errorf("unexpected response:%+v", resp)
	}

	return nil
}

func (s *MobileImageMounterService) Umount() error {
	req := map[string]interface{}{
		"Command":   "UnmountImage",
		"MountPath": "/Developer", // only need for older iOS
		// "ImageSignature": signature,
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return err
	}

	if v, ok := resp["Error"]; ok {
		if v == "UnknownCommand" {
			return ErrUnsupported
		} else if v == "InternalError" {
			return ErrInternal
		} else {
			return ErrNotMounted
		}
	}

	return nil
}

// uploadImage upload image into device.
func (s *MobileImageMounterService) uploadImage(imagePath, signaturePath string) error {
	signature, err := os.ReadFile(signaturePath)
	if err != nil {
		return err
	}

	info, err := os.Stat(imagePath)
	if err != nil {
		return err
	}
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	req := map[string]interface{}{
		"Command":        "ReceiveBytes",
		"ImageSignature": signature,
		"ImageSize":      info.Size(),
		"ImageType":      "Developer",
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return err
	}
	status, ok := resp["Status"]
	if !ok || status != "ReceiveBytesAck" {
		return fmt.Errorf("unexpected response:%+v", resp)
	}

	// send image
	_, err = io.Copy(s.Conn.Writer(), imageFile)
	if err != nil {
		return err
	}
	resp, err = s.Receive()
	if err != nil {
		return err
	}

	status, ok = resp["Status"]
	if !ok || status != "Complete" {
		return fmt.Errorf("unexpected response:%+v", resp)
	}
	return nil
}
