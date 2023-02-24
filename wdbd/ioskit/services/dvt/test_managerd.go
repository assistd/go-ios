package dvt

import (
	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
)

type TestManagerdSecureService struct {
	services.RemoteServer
}

func NewTestManagerdSecureService(device ios.DeviceEntry) (*TestManagerdSecureService, error) {
	const service = "com.apple.testmanagerd.lockdown.secure"
	const serviceBeforeIOS14 = "com.apple.testmanagerd.lockdown"

	var name string
	version, err := ios.GetProductVersion(device)
	if err != nil {
		return nil, err
	}
	if version.LessThan(semver.MustParse("14.0")) {
		name = serviceBeforeIOS14
	} else {
		name = service
	}

	s := &TestManagerdSecureService{}
	s.Name = name
	s.IsDeveloper = true

	err = s.Init(device)
	return s, err
}

func (r *TestManagerdSecureService) GetXcodeIDEChannel() services.Channel {
	return services.BuildChannel(&r.RemoteServer, 0xFFFFFFFF) //-1
}
