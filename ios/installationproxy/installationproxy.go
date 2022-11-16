package installationproxy

import (
	"bytes"
	"context"
	"fmt"
	ios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
	"os"
)

const serviceName = "com.apple.mobile.installation_proxy"

type InstallationProxyCmd string

const (
	CmdInstall   InstallationProxyCmd = "Install"
	CmdUnInstall InstallationProxyCmd = "Uninstall"
)

type Connection struct {
	deviceConn ios.DeviceConnectionInterface
	plistCodec ios.PlistCodec
}

func (c *Connection) Close() {
	c.deviceConn.Close()
}

func New(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, serviceName)
	if err != nil {
		return &Connection{}, err
	}
	return &Connection{deviceConn: deviceConn, plistCodec: ios.NewPlistCodec()}, nil
}
func (conn *Connection) BrowseUserApps() ([]AppInfo, error) {
	return conn.browseApps(browseApps("User", true))
}

func (conn *Connection) BrowseSystemApps() ([]AppInfo, error) {
	return conn.browseApps(browseApps("System", false))
}

func (conn *Connection) BrowseAllApps() ([]AppInfo, error) {
	return conn.browseApps(browseApps("", true))
}

func (conn *Connection) BrowseAnyApps() ([]AppInfo, error) {
	return conn.browseApps(browseAnyApps())
}

func (conn *Connection) browseApps(request interface{}) ([]AppInfo, error) {
	reader := conn.deviceConn.Reader()
	bytes, err := conn.plistCodec.Encode(request)
	if err != nil {
		return make([]AppInfo, 0), err
	}
	conn.deviceConn.Send(bytes)
	stillReceiving := true
	responses := make([]BrowseResponse, 0)
	size := uint64(0)
	for stillReceiving {
		response, err := conn.plistCodec.Decode(reader)
		ifa, err := plistFromBytes(response)
		stillReceiving = "Complete" != ifa.Status
		if err != nil {
			return make([]AppInfo, 0), err
		}
		size += ifa.CurrentAmount
		responses = append(responses, ifa)
	}
	appinfos := make([]AppInfo, size)

	for _, v := range responses {
		copy(appinfos[v.CurrentIndex:], v.CurrentList)

	}
	return appinfos, nil
}

func (c *Connection) Install(device ios.DeviceEntry, path string, bundleId string) error {
	ctx, cancel := context.WithCancel(context.Background())
	err := c.InstallWithCtx(ctx, device, path, bundleId, func(event ios.InstallEvent) {
		if event.Stage == ios.InstallByPushDir {
			log.Infof("Copying %v to device... %d / 100", path, event.Percent)
		}
	})
	cancel()
	return err
}

func (c *Connection) InstallWithCtx(ctx context.Context, device ios.DeviceEntry, path string, bundleId string,
	notify func(event ios.InstallEvent)) error {
	if len(bundleId) == 0 {
		infoPlist, err := ios.GetInfoPlistFromIpa(path)
		if err != nil {
			return err
		}

		if len(infoPlist.CFBundleIdentifier) == 0 {
			return fmt.Errorf("cannot find CFBundleIdentifier in Info.plist")
		}

		bundleId = infoPlist.CFBundleIdentifier
	}

	afcService, err := afc.New(device)
	log.Infof("Copying %v to device...", path)

	ipaTmpDir := "PublicStaging"
	if _, err := afcService.Stat(ipaTmpDir); err != nil {
		afcService.MakeDir(ipaTmpDir)
	}
	targetPath := fmt.Sprintf("%v/%v.ipa", ipaTmpDir, bundleId)
	go func() {
		<-ctx.Done()
		afcService.Close()
	}()
	if notify != nil {
		ctx2, cancel := context.WithCancel(ctx)
		ipaFile, err := os.Stat(path)
		if err != nil {
			return err
		}
		ipaFileSize := uint64(ipaFile.Size())
		listener := ios.PushListener{
			OverallSize: ipaFileSize,
			IpaFileSize: ipaFileSize,
		}
		go listener.Start(ctx2, notify)

		err = afcService.PushWithWriter(path, targetPath, &listener)
		if err != nil {
			return err
		}
		if cancel != nil {
			cancel()
		}
	} else {
		err = afcService.Push(path, targetPath)
		if err != nil {
			return err
		}
	}
	log.Infof("Done.")
	return c.install(ctx, bundleId, targetPath, notify)
}

func (c *Connection) install(ctx context.Context, bundleId, path string, notify func(event ios.InstallEvent)) error {
	options := map[string]interface{}{
		"CFBundleIdentifier": bundleId,
	}
	installCommand := map[string]interface{}{
		"Command":       CmdInstall,
		"ClientOptions": options,
		"PackagePath":   path,
	}
	b, err := c.plistCodec.Encode(installCommand)
	if err != nil {
		return err
	}
	err = c.deviceConn.Send(b)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		c.deviceConn.Close()
	}()
	for {
		response, err := c.plistCodec.Decode(c.deviceConn.Reader())
		if err != nil {
			return err
		}
		dict, err := ios.ParsePlist(response)
		if err != nil {
			return err
		}
		done, err := checkFinished(dict, CmdInstall)
		if notify != nil {
			if statusIntf, ok := dict["Status"]; ok {
				percentIntf, ok := dict["PercentComplete"]
				if ok {
					notify(ios.InstallEvent{Stage: statusIntf.(string), Percent: int(percentIntf.(uint64))})
				}
			}
		}
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func (c *Connection) Uninstall(bundleId string) error {
	options := map[string]interface{}{}
	uninstallCommand := map[string]interface{}{
		"Command":               CmdUnInstall,
		"ApplicationIdentifier": bundleId,
		"ClientOptions":         options,
	}
	b, err := c.plistCodec.Encode(uninstallCommand)
	if err != nil {
		return err
	}
	err = c.deviceConn.Send(b)
	if err != nil {
		return err
	}
	for {
		response, err := c.plistCodec.Decode(c.deviceConn.Reader())
		if err != nil {
			return err
		}
		dict, err := ios.ParsePlist(response)
		if err != nil {
			return err
		}
		done, err := checkFinished(dict, CmdUnInstall)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func checkFinished(dict map[string]interface{}, c InstallationProxyCmd) (bool, error) {
	if val, ok := dict["Error"]; ok {
		return true, fmt.Errorf("received %v error: %v", c, val)
	}
	if val, ok := dict["Status"]; ok {
		if "Complete" == val {
			log.Infof("done %v", c)
			return true, nil
		}
		log.Infof("%v status: %s", c, val)
		return false, nil
	}
	return true, fmt.Errorf("unknown status update: %+v", dict)
}

func plistFromBytes(plistBytes []byte) (BrowseResponse, error) {
	var browseResponse BrowseResponse
	decoder := plist.NewDecoder(bytes.NewReader(plistBytes))

	err := decoder.Decode(&browseResponse)
	if err != nil {
		return browseResponse, err
	}
	return browseResponse, nil
}
func browseApps(applicationType string, showLaunchProhibitedApps bool) map[string]interface{} {
	returnAttributes := []string{
		"ApplicationDSID",
		"ApplicationType",
		"CFBundleDisplayName",
		"CFBundleExecutable",
		"CFBundleIdentifier",
		"CFBundleName",
		"CFBundleShortVersionString",
		"CFBundleVersion",
		"Container",
		"Entitlements",
		"EnvironmentVariables",
		"MinimumOSVersion",
		"Path",
		"ProfileValidated",
		"SBAppTags",
		"SignerIdentity",
		"UIDeviceFamily",
		"UIRequiredDeviceCapabilities",
		"UIFileSharingEnabled",
	}
	clientOptions := map[string]interface{}{
		"ReturnAttributes": returnAttributes,
	}
	if applicationType != "" {
		clientOptions["ApplicationType"] = applicationType
	}
	if showLaunchProhibitedApps {
		clientOptions["ShowLaunchProhibitedApps"] = true
	}
	return map[string]interface{}{"ClientOptions": clientOptions, "Command": "Browse"}
}

func browseAnyApps() map[string]interface{} {
	returnAttributes := []string{
		"CFBundleIdentifier",
		"CFBundleDisplayName",
		"CFBundleVersion",
		"UIFileSharingEnabled",
	}
	clientOptions := map[string]interface{}{
		"ApplicationType":  "Any",
		"ReturnAttributes": returnAttributes,
	}
	return map[string]interface{}{"ClientOptions": clientOptions, "Command": "Browse"}
}

type BrowseResponse struct {
	CurrentIndex  uint64
	CurrentAmount uint64
	Status        string
	CurrentList   []AppInfo
}
type AppInfo struct {
	ApplicationDSID              int
	ApplicationType              string
	CFBundleDisplayName          string
	CFBundleExecutable           string
	CFBundleIdentifier           string
	CFBundleName                 string
	CFBundleShortVersionString   string
	CFBundleVersion              string
	Container                    string
	Entitlements                 map[string]interface{}
	EnvironmentVariables         map[string]interface{}
	MinimumOSVersion             string
	Path                         string
	ProfileValidated             bool
	SBAppTags                    []string
	SignerIdentity               string
	UIDeviceFamily               []int
	UIRequiredDeviceCapabilities []string
	UIFileSharingEnabled         bool
}
