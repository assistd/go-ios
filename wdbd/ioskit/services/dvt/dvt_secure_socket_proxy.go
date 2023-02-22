package dvt

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
)

type DvtSecureSocketProxyService struct {
	services.RemoteServer
}

func NewDvtSecureSocketProxyService(device ios.DeviceEntry) (*DvtSecureSocketProxyService, error) {
	const serviceIOS14 = "com.apple.instruments.remoteserver.DVTSecureSocketProxy"
	const serviceOldName = "com.apple.instruments.remoteserver"

	s := &DvtSecureSocketProxyService{}
	s.Name = serviceIOS14
	s.IsDeveloper = true

	err := s.Init(device)
	return s, err
}
