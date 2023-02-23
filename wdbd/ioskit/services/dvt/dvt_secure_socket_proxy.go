package dvt

import (
	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit/services"
)

type DvtSecureSocketProxyService struct {
	services.RemoteServer
}

func NewDvtSecureSocketProxyService(device ios.DeviceEntry) (*DvtSecureSocketProxyService, error) {
	// iOS14后系统的本服务全程使用ssl加密，而iOS14以前在ssl握手后使用明文传输
	// pymobileservice3中对应的是remove_ssl_context参数标识
	// go-ios的devConn已内置，执行s.Init -> ConnectToService时会根据服务名自动处理
	const service = "com.apple.instruments.remoteserver.DVTSecureSocketProxy"
	const serviceBeforeIOS14 = "com.apple.instruments.remoteserver"

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

	s := &DvtSecureSocketProxyService{}
	s.Name = name
	s.IsDeveloper = true

	err = s.Init(device)
	return s, err
}
