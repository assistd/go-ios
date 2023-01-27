package mackit

import (
	"bytes"
	"fmt"
	"html/template"
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

// 说明
// 当前实现为快速实现功能，并不优雅也不通用
// defatuts命令的输出格式与xcode的工程文件pbxproj格式类似，都是类似ini形式的变形，后续有空实现个解析该格式的通用解码/编码库
// 参考：
//  https://github.com/go-ini/ini
//  https://github.com/st3fan/pbxproj

/*
   {
       bonjourServiceName = "08:ff:44:b0:e2:a1@fe80::aff:44ff:feb0:e2a1._apple-mobdev2._tcp.local.";
       buildVersion = 19H12;
       canBeWatchCompanion = 0;
       deviceActivationState = Activated;
       deviceArchitecture = arm64e;
       deviceAvailableCapacity = 48193527808;
       deviceBatteryCapacity = 100;
       deviceBluetoothMAC = "08:ff:44:ad:02:b7";
       deviceChipID = 32789;
       deviceClass = iPhone;
       deviceCodename = D22AP;
       deviceColorString = 1;
       deviceDevelopmentStatus = Development;
       deviceECID = 2587430847496238;
       deviceEnclosureColorString = 2;
       deviceIMEI = 353058092195610;
       deviceIdentifier = "00008110-00142C9A3642801E";
       deviceIsProduction = 1;
       deviceName = "iPad mini";
       deviceProductionSOC = 1;
       deviceSerialNumber = FQLQ9R93NV;
       deviceToolsType = None;
       deviceTotalCapacity = 59187580928;
       deviceType = "iPad14,1";
       deviceWiFiMAC = "08:ff:44:b0:e2:a1";
       isPasscodeLocked = 0;
       isWirelessEnabled = 1;
       platformIdentifier = "com.apple.platform.iphoneos";
       productVersion = "15.7";
       tokenClass = DTDKMobileDeviceToken;
       wakeupToken =         {
           FullServiceNameKey = "08:ff:44:b0:e2:a1@fe80::72ea:5aff:fe2a:88ad._apple-mobdev._tcp.local.";
           InterfaceIndexKey = 16;
           TokenVersionKey = 4;
       };
*/

const deviceTokenTemp = `{
	bonjourServiceName = "{{.WiFiAddress}}@fe80::aff:44ff:feb0:e2a1._apple-mobdev2._tcp.local.";
	buildVersion = {{.BuildVersion}};
	canBeWatchCompanion = 0;
	deviceActivationState = Activated;
	deviceArchitecture = {{.DeviceArchitecture}};
	deviceAvailableCapacity = 48193527808;
	deviceBatteryCapacity = 100;
	deviceBluetoothMAC = "{{.DeviceBluetoothMAC}}";
	deviceChipID = {{.ChipID}};
	deviceClass = iPhone;
	deviceCodename = {{.HardwareModel}};
	deviceColorString = 1;
	deviceDevelopmentStatus = Development;
	deviceECID = 2587430847496238;
	deviceEnclosureColorString = 2;
	deviceIMEI = 353058092195610;
	deviceIdentifier = "{{.UUID}}";
	deviceIsProduction = 1;
	deviceName = "{{.DeviceName}}";
	deviceProductionSOC = 1;
	deviceSerialNumber = {{.Serial}};
	deviceToolsType = None;
	deviceTotalCapacity = 59187580928;
	deviceType = "{{.DeviceType}}";
	deviceWiFiMAC = "{{.WiFiAddress}}";
	isPasscodeLocked = 0;
	isWirelessEnabled = 1;
	platformIdentifier = "com.apple.platform.iphoneos";
	productVersion = "{{.ProductVersion}}";
	tokenClass = DTDKMobileDeviceToken;
	wakeupToken =         {
		FullServiceNameKey = "{{.WiFiAddress}}@fe80::72ea:5aff:fe2a:88ad._apple-mobdev._tcp.local.";
		InterfaceIndexKey = 16;
		TokenVersionKey = 4;
	};
}`

type DeviceInfo struct {
	UUID               string
	Serial             string
	BuildVersion       string
	DeviceArchitecture string
	WiFiAddress        string
	DeviceBluetoothMAC string
	DeviceName         string
	DeviceType         string
	ProductVersion     string
	HardwareModel      string
	ChipID             int
}

func FindDevice(uuid string) bool {
	cmdStr := "defaults read com.apple.dt.Xcode DVTDeviceTokens"
	cmdArray := strings.Split(cmdStr, " ")
	cmd := exec.Command(cmdArray[0], cmdArray[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), uuid)
}

func BuildDeviceInfo(allValues map[string]interface{}) DeviceInfo {
	entry := DeviceInfo{}
	entry.UUID = allValues["UniqueDeviceID"].(string)
	entry.Serial = allValues["SerialNumber"].(string)
	entry.BuildVersion = allValues["BuildVersion"].(string)
	entry.WiFiAddress = allValues["WiFiAddress"].(string)
	entry.DeviceType = allValues["ProductType"].(string)
	entry.DeviceName = allValues["ProductName"].(string)
	entry.ProductVersion = allValues["ProductVersion"].(string)
	entry.HardwareModel = allValues["HardwareModel"].(string)
	entry.DeviceArchitecture = allValues["CPUArchitecture"].(string)
	entry.DeviceBluetoothMAC = allValues["BluetoothAddress"].(string)
	entry.ChipID = allValues["ChipID"].(int)
	return entry
}

func AddDevice(info DeviceInfo) error {
	templ, err := template.New("deviceToken").Parse(deviceTokenTemp)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	err = templ.Execute(buf, info)
	if err != nil {
		return err
	}

	fmt.Println("device token: ", buf.String())

	cmd := exec.Command("defaults", "write", "com.apple.dt.Xcode", "DVTDeviceTokens", "-array-add", buf.String())
	_, err = cmd.Output()
	return err
}

func RemoveDevice(uuid string) error {
	// TODO
	return nil
}

func ReadWirelessBuddyID() (string, error) {
	cmd := exec.Command("defaults", "read", "com.apple.iTunes", "WirelessBuddyID")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func AllocateWriteWirelessID() string {
	return strings.ToUpper(uuid.New().String())
}

func WriteWirelessBuddyID(uuidStr string) error {
	cmd := exec.Command("defaults", "write", "com.apple.iTunes", "WirelessBuddyID", uuidStr)
	_, err := cmd.Output()
	if err != nil {
		return err
	}

	return nil
}

func WriteWirelessBuddyID2() (string, error) {
	uuidStr := strings.ToUpper(uuid.New().String())
	err := WriteWirelessBuddyID(uuidStr)
	if err != nil {
		return "", err
	}

	return uuidStr, nil
}
