package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/tunnel"
)

type AmfiService struct {
	BaseService
}

func NewAmfiService(device ios.DeviceEntry) (*AmfiService, error) {
	s := &AmfiService{
		BaseService: BaseService{
			Name:        "com.apple.amfi.lockdown",
			IsDeveloper: false,
		},
	}
	err := s.init(device)
	return s, err
}

func (s *AmfiService) Init() error {
	return nil
}

func (s *AmfiService) EnableDeveloperMode(enablePostRestart bool) error {
	req := map[string]interface{}{
		"action": 1,
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return err
	}

	status, ok := resp["Error"]
	if ok {
		return errors.New(status.(string))
	}

	_, ok = resp["success"]
	if !ok {
		return fmt.Errorf("enable_developer_mode failed: %+v", resp)
	}

	if !enablePostRestart {
		return nil
	}

	// NewHeartbeatServiceWithConn(s.Conn).Start()
	// wait reboot ok
	var device ios.DeviceEntry
	for {
		device, err = tunnel.GetDevice(s.udid)
		if err == nil && device.DeviceID != s.deviceID {
			c, err := ios.ConnectLockdownWithSession(device)
			if err == nil {
				c.Close()
				break
			}
		}

		time.Sleep(time.Second)
	}

	// hack to reinit device
	s.inited = false
	err = s.init(device)
	if err != nil {
		return err
	}

	return s.EnableDeveloperModePostRestart()
}

// EnableDeveloperModePostRestart answer the prompt that appears after the restart with "yes"
func (s *AmfiService) EnableDeveloperModePostRestart() error {
	req := map[string]interface{}{
		"action": 2,
	}
	resp, err := s.SendReceive(req)
	if err != nil {
		return err
	}
	_, ok := resp["success"]
	if !ok {
		return fmt.Errorf("enable_developer_mode_post_restart failed: %+v", resp)
	}
	return nil
}
