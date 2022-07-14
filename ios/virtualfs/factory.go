package virtualfs

import (
	"github.com/danielpaulus/go-ios/ios"
)

func NewIosFs(udid string) (VirtualFs, error) {
	device, err := ios.GetDevice(udid)
	if err != nil {
		return nil, err
	}

	rootFs := &VirtualRootFs{device, make(map[string]VirtualFs)}
	rootFs.DoMount("/FileSystem", NewDeviceFs(udid, "/FileSystem"))
	rootFs.DoMount("/AppSandboxes", NewVirtualSandboxGroupFs(udid, "/AppSandboxes"))
	return rootFs, nil
}
