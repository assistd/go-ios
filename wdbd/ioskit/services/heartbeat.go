package services

import (
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

// HeartbeatService use to keep an active connection with lockdowd
type HeartbeatService struct {
	BaseService
}

func NewHeartbeatService(device ios.DeviceEntry) (*HeartbeatService, error) {
	s := &HeartbeatService{
		BaseService: BaseService{
			Name:        "com.apple.mobile.heartbeat",
			IsDeveloper: false,
		},
	}
	err := s.init(device)
	return s, err
}

func NewHeartbeatServiceWithConn(conn ios.DeviceConnectionInterface) *HeartbeatService {
	s := &HeartbeatService{
		BaseService: BaseService{
			Name:   "com.apple.mobile.heartbeat",
			Conn:   conn,
			inited: true,
		},
	}
	return s
}

func (s *HeartbeatService) Start() error {
	req := map[string]interface{}{
		"Command": "Polo",
	}
	for {
		resp, err := s.Receive()
		if err != nil {
			return err
		}
		log.Infof("heartbeat: %#v", resp)
		err = s.Send(req)
		if err != nil {
			return err
		}
	}
}
