package installationproxy

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"

	ios "github.com/danielpaulus/go-ios/ios"
	"howett.net/plist"
)

const serviceName = "com.apple.mobile.installation_proxy"

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
	return conn.browseApps(browseUserApps())
}

func (conn *Connection) BrowseSystemApps() ([]AppInfo, error) {
	return conn.browseApps(browseSystemApps())
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

func (c *Connection) Uninstall(bundleId string) error {
	options := map[string]interface{}{}
	uninstallCommand := map[string]interface{}{
		"Command":               "Uninstall",
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
		done, err := checkFinished(dict)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func checkFinished(dict map[string]interface{}) (bool, error) {
	if val, ok := dict["Error"]; ok {
		return true, fmt.Errorf("received uninstall error: %v", val)
	}
	if val, ok := dict["Status"]; ok {
		if "Complete" == val {
			log.Info("done uninstalling")
			return true, nil
		}
		log.Infof("uninstall status: %s", val)
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
func browseSystemApps() map[string]interface{} {
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
	}
	clientOptions := map[string]interface{}{
		"ApplicationType":  "System",
		"ReturnAttributes": returnAttributes,
	}
	return map[string]interface{}{"ClientOptions": clientOptions, "Command": "Browse"}
}

func browseUserApps() map[string]interface{} {
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
		"ApplicationType":          "User",
		"ReturnAttributes":         returnAttributes,
		"ShowLaunchProhibitedApps": true,
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
