package ios

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	plist "howett.net/plist"
)

//ToPlist converts a given struct to a Plist using the
//github.com/DHowett/go-plist library. Make sure your struct is exported.
//It returns a string containing the plist.
func ToPlist(data interface{}) string {
	return string(ToPlistBytes(data))
}

//ParsePlist tries to parse the given bytes, which should be a Plist, into a map[string]interface.
//It returns the map or an error if the decoding step fails.
func ParsePlist(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	_, err := plist.Unmarshal(data, &result)
	return result, err
}

//ToPlistBytes converts a given struct to a Plist using the
//github.com/DHowett/go-plist library. Make sure your struct is exported.
//It returns a byte slice containing the plist.
func ToPlistBytes(data interface{}) []byte {
	bytes, err := plist.Marshal(data, plist.XMLFormat)
	if err != nil {
		//this should not happen
		panic(fmt.Sprintf("Failed converting to plist %v error:%v", data, err))
	}
	return bytes
}

//Ntohs is a re-implementation of the C function Ntohs.
//it means networkorder to host oder and basically swaps
//the endianness of the given int.
//It returns port converted to little endian.
func Ntohs(port uint16) uint16 {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, port)
	return binary.LittleEndian.Uint16(buf)
}

//GetDevice returns:
// the device for the udid if a valid udid is provided.
// if the env variable 'udid' is specified, the device with that udid
// otherwise it returns the first device in the list.
func GetDevice(udid string) (DeviceEntry, error) {
	if udid == "" {
		udid = os.Getenv("udid")
		if udid != "" {
			log.Info("using udid from env.udid variable")
		}
	}
	log.Debugf("Looking for device '%s'", udid)
	deviceList, err := ListDevices()
	if err != nil {
		return DeviceEntry{}, err
	}
	if udid == "" {
		if len(deviceList.DeviceList) == 0 {
			return DeviceEntry{}, errors.New("no iOS devices are attached to this host")
		}
		log.WithFields(log.Fields{"udid": deviceList.DeviceList[0].Properties.SerialNumber}).
			Info("no udid specified using first device in list")
		return deviceList.DeviceList[0], nil
	}
	for _, device := range deviceList.DeviceList {
		if device.Properties.SerialNumber == udid {
			return device, nil
		}
	}
	return DeviceEntry{}, fmt.Errorf("Device '%s' not found. Is it attached to the machine?", udid)
}

//PathExists is used to determine whether the path folder exists
//True if it exists, false otherwise
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func IOS14() *semver.Version {
	return semver.MustParse("14.0")
}

func IOS12() *semver.Version {
	return semver.MustParse("12.0")
}

func IOS11() *semver.Version {
	return semver.MustParse("11.0")
}

//FixWindowsPaths replaces backslashes with forward slashes and removes the X: style
//windows drive letters
func FixWindowsPaths(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.Contains(path, ":/") {
		path = strings.Split(path, ":/")[1]
	}
	return path
}

type InfoPlist struct {
	CFBundleName         string `plist:"CFBundleName"`
	CFBundleDisplayName  string `plist:"CFBundleDisplayName"`
	CFBundleVersion      string `plist:"CFBundleVersion"`
	CFBundleShortVersion string `plist:"CFBundleShortVersionString"`
	CFBundleIdentifier   string `plist:"CFBundleIdentifier"`
}

func getFileInfoFromIpa(ipaPath string, r *regexp.Regexp) ([]byte, error) {
	if r == nil {
		return nil, errors.New("reInfoPlist is nil")
	}
	ipaFile, err := os.Open(ipaPath)
	if err != nil {
		return nil, err
	}
	defer ipaFile.Close()

	stat, err := ipaFile.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(ipaFile, stat.Size())
	if err != nil {
		return nil, err
	}

	var file *zip.File
	for _, f := range reader.File {
		if file == nil {
			switch {
			case r.MatchString(f.Name):
				file = f
			}
		} else {
			break
		}
	}

	if file == nil {
		return nil, errors.New("file not found")
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func GetInfoPlistFromIpa(ipaPath string) (*InfoPlist, error) {
	r := regexp.MustCompile(`Payload/[^/]+/Info\.plist`)
	infoPlistFile, err := getFileInfoFromIpa(ipaPath, r)
	if err != nil {
		return nil, err
	}

	p := new(InfoPlist)
	decoder := plist.NewDecoder(bytes.NewReader(infoPlistFile))
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}
	return p, nil
}

const (
	InstallByConduitZip = "InstallByConduitZip"
	InstallByPushDir    = "InstallByPushDir"
	DefRefrashRate      = time.Second
)

type InstallEvent struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
	Current int64  `json:"current"`
	Total   int64  `json:"total"`
	Speed   int64  `json:"speed"`
}

type PushListener struct {
	currentSize   uint64
	lastTotalSize uint64
	IpaFileSize   uint64
	OverallSize   uint64
	finishPush    bool
}

func (u *PushListener) Write(b []byte) (n int, err error) {
	u.currentSize = u.currentSize + uint64(len(b))
	u.lastTotalSize = u.lastTotalSize + uint64(len(b))
	return len(b), nil
}

func (u *PushListener) Start(ctx context.Context, notify func(event InstallEvent)) {
	u.currentSize = 0

	refresh := func(finish bool) {
		if u.finishPush {
			return
		}

		f := float64(u.currentSize) * float64((time.Second.Milliseconds())/DefRefrashRate.Milliseconds())
		percent := float64(u.lastTotalSize) / float64(u.OverallSize)
		if finish {
			percent = 1
		}

		notify(InstallEvent{
			Stage:   InstallByPushDir,
			Current: int64(float64(u.IpaFileSize) * percent),
			Total:   int64(u.IpaFileSize),
			Speed:   int64(f),
			Percent: int(percent * 100),
		})
		u.currentSize = 0
		u.finishPush = percent == 1
	}

	for {
		select {
		case <-ctx.Done():
			refresh(true)
			return
		case <-time.After(DefRefrashRate):
			refresh(false)
		}
	}
}
