package zipconduit

import (
	"context"
	"encoding/binary"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

/**
Typical weird iOS service :-D
It is a kind of special "zip" format that XCode uses to send files&folder to devices.
Sadly it is not compliant with all standard zip libraries, in particular it does not work
with the golang zipWriter implementation... OF COURSE ;-)
This is why I had to hack my own "zip" encoding together. Here is how zip_conduit works:

1. Send PLIST "InitTransfer" in standard 4byte length + Plist format
2. Start sending binary zip stream next
3. Since zip does not support streaming,
   we first generate a metainf file inside a metainf directory. It contains number of files and
   total byte sizes among other things (check the struct). Probably to make streaming work we also send
   it as the first file
4. Starting with metainf for each file:
       send a ZipFileHeader with compression set to STORE (so no compression at all)
       this also means uncompressedSize==compressedSize btw.
       be sure not to use DataDescriptors (https://en.wikipedia.org/wiki/ZIP_(file_format)#Local_file_header)
       I guess they have disabled them as it would make streaming harder. This is why golang's zip implementation
       does not work.
5. Send the standard central directory header but not a central directory (obviously)
6. wait for a bunch of PLISTs to be received that indicate progress and completion of installation
*/
const serviceName string = "com.apple.streaming_zip_conduit"

//Connection exposes functions to interoperate with zipconduit
type Connection struct {
	deviceConn ios.DeviceConnectionInterface
	plistCodec ios.PlistCodec
}

//New returns a new ZipConduit Connection for the given DeviceID and Udid
func New(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, serviceName)
	if err != nil {
		return &Connection{}, err
	}

	return &Connection{
		deviceConn: deviceConn,
		plistCodec: ios.NewPlistCodec(),
	}, nil
}

//SendFile will send either a zipFile or an unzipped directory to the device.
//If you specify appFilePath to a file, it will try to Unzip it to a temp dir first and then send.
//If appFilePath points to a directory, it will try to install the dir contents as an app.

func (conn Connection) SendFile(appFilePath string) error {
	return conn.SendFileWithProgress(appFilePath, nil, nil)
}

func (conn Connection) SendFileWithProgress(appFilePath string, ctx context.Context, notify func(event InstallEvent)) error {
	openedFile, err := os.Open(appFilePath)
	if err != nil {
		return err
	}
	// Get the file information
	info, err := openedFile.Stat()
	openedFile.Close()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return conn.sendDirectoryWithProgress(appFilePath, notify)
	}

	isCondZip, err := IsConduitZip(appFilePath)
	if isCondZip {
		return conn.InstallConduitAppWithProgress(appFilePath, ctx, notify)
	}

	return conn.InstallIpaAppWithProgress(appFilePath, ctx, notify)
}

func (conn Connection) sendDirectory(dir string, notify func(event InstallEvent)) error {
	return conn.sendDirectoryWithProgress(dir, nil)
}

func (conn Connection) sendDirectoryWithProgress(dir string, notify func(event InstallEvent)) error {
	err := conn.initTransfer(dir + ".ipa")
	if err != nil {
		return err
	}
	deviceStream := conn.deviceConn.Writer()
	err = packDirToConduitStream(dir, deviceStream)
	if err != nil {
		return err
	}
	return conn.waitForInstallationWithProgress(notify)
}

func (conn Connection) InstallIpaApp(ipaApp string) error {
	return conn.InstallIpaAppWithProgress(ipaApp, nil, nil)
}

func (conn Connection) InstallIpaAppWithProgress(ipaApp string, ctx context.Context, notify func(event InstallEvent)) error {
	pwd, _ := os.Getwd()
	tmpDir, err := ioutil.TempDir(pwd, "temp")
	if err != nil {
		return err
	}

	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.WithFields(log.Fields{"dir": tmpDir}).Warn("failed removing tempdir")
		}
	}()

	ipaFile,_ := os.Stat(ipaApp)
	ipaFileSize := uint64(ipaFile.Size())

	var overallSize uint64
	_, overallSize, err = Unzip(ipaApp, tmpDir)
	if err != nil {
		return err
	}

	err = conn.initTransfer(ipaApp)
	if err != nil {
		return err
	}

	deviceStream := conn.deviceConn.Writer()
	dst := deviceStream

	var ctx2 context.Context
	var cancel context.CancelFunc
	var listener PushListener
	if ctx != nil {
		ctx2, cancel = context.WithCancel(ctx)
		listener = PushListener{
			overallSize:   overallSize,
			ipaFileSize:   ipaFileSize,
		}
	}

	if notify != nil {
		go listener.Start(ctx2, notify)
		dst = io.MultiWriter(deviceStream, &listener)
	}

	err = packDirToConduitStream(tmpDir, dst)
	if ctx2 != nil {
		cancel()
	}
	if err != nil {
		return err
	}

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				conn.deviceConn.Close()
				break
			}
		}()
	}
	return conn.waitForInstallationWithProgress(notify)
}

func (conn Connection) waitForInstallationWithProgress(notify func(event InstallEvent)) error {
	for {
		msg, _ := conn.plistCodec.Decode(conn.deviceConn.Reader())
		plist, _ := ios.ParsePlist(msg)
		log.Debugf("%+v", plist)
		done, percent, status, err := evaluateProgress(plist)
		if err != nil {
			return err
		}
		if done {
			if notify != nil {
				notify(InstallEvent{Stage: "install successful", Percent: 100})
			}
			log.Info("installation successful")
			return nil
		} else {
			if notify != nil {
				notify(InstallEvent{Stage: status, Percent: percent})
			}
		}
		log.WithFields(log.Fields{"status": status, "percentComplete": percent}).Info("installing")
	}
}

const metainfFileName = "com.apple.ZipMetadata.plist"

func addMetaInf(metainfPath string, files []string, totalBytes uint64) (string, string, error) {
	folderPath := path.Join(metainfPath, "META-INF")
	ret, _ := ios.PathExists(folderPath)
	if !ret {
		err := os.Mkdir(folderPath, 0777)
		if err != nil {
			return "", "", err
		}
	}
	//recordcount == files + meta-inf + metainffile
	meta := metadata{RecordCount: 2 + len(files), StandardDirectoryPerms: 16877, StandardFilePerms: -32348, TotalUncompressedBytes: totalBytes, Version: 2}
	metaBytes := ios.ToPlistBytes(meta)
	filePath := path.Join(metainfPath, "META-INF", metainfFileName)
	err := ioutil.WriteFile(filePath, metaBytes, 0777)
	if err != nil {
		return "", "", err
	}
	return folderPath, filePath, nil
}

func AddFileToZip(writer io.Writer, filename string, tmpdir string) error {
	fileToZip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	// Get the file information
	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		filename = strings.ReplaceAll(filename, "\\", "/")
		tmpdir = strings.ReplaceAll(tmpdir, "\\", "/")
	}

	// Using FileInfoHeader() above only uses the basename of the file. If we want
	// to preserve the folder structure we can overwrite this with the full path.
	filenameForZip := strings.Replace(filename, tmpdir+"/", "", 1)
	if info.IsDir() && !strings.HasSuffix(filenameForZip, "/") {
		filenameForZip += "/"
	}
	if info.IsDir() {
		//write our "zip" header for a directory
		header, name, extra := newZipHeaderDir(filenameForZip)
		err := binary.Write(writer, binary.LittleEndian, header)
		if err != nil {
			return err
		}
		err = binary.Write(writer, binary.BigEndian, name)
		if err != nil {
			return err
		}
		err = binary.Write(writer, binary.BigEndian, extra)
		return err
	}

	crc, err := calculateCrc32(fileToZip)
	if err != nil {
		return err
	}
	fileToZip.Seek(0, io.SeekStart)
	//write our "zip" file header
	header, name, extra := newZipHeader(uint32(info.Size()), crc, filenameForZip)
	err = binary.Write(writer, binary.LittleEndian, header)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, name)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, extra)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, fileToZip)
	return err
}

func calculateCrc32(reader io.Reader) (uint32, error) {
	hash := crc32.New(crc32.IEEETable)
	if _, err := io.Copy(hash, reader); err != nil {
		return 0, err
	}
	return hash.Sum32(), nil
}

func (conn Connection) InstallConduitApp(conduitApp string) error {
	return conn.InstallConduitAppWithProgress(conduitApp, nil, nil)
}

func (conn Connection) InstallConduitAppWithProgress(conduitApp string, ctx context.Context, notify func(event InstallEvent)) error {
	reader, err := os.Open(conduitApp)
	if err != nil {
		return err
	}
	defer reader.Close()

	if notify != nil {
		notify(InstallEvent{Stage: InstallByConduitZip, Percent: 0})
	}

	// skip header
	header := &conduitZipHeader{}
	err = binary.Read(reader, binary.BigEndian, header)
	if err != nil {
		return err
	}

	err = conn.initTransfer(conduitApp)
	if err != nil {
		return err
	}
	deviceStream := conn.deviceConn.Writer()
	_, err = io.Copy(deviceStream, reader)
	if err != nil {
		return err
	}
	return conn.waitForInstallationWithProgress(notify)
}

func (conn Connection) initTransfer(ipaApp string) error {
	init := newInitTransfer(ipaApp)
	log.Debug("sending inittransfer %+v", init)
	bytes, _ := conn.plistCodec.Encode(init)

	err := conn.deviceConn.Send(bytes)
	if err != nil {
		return err
	}
	return nil
}

const (
	InstallByConduitZip = "InstallByConduitZip"
	InstallByPushDir    = "InstallByPushDir"
	InstallByIPAUnzip   = "InstallByIPAUnzip"
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
	ipaFileSize   uint64
	overallSize   uint64
}

func (u *PushListener) Write(b []byte) (n int, err error) {
	u.currentSize = u.currentSize + uint64(len(b))
	u.lastTotalSize =  u.lastTotalSize + uint64(len(b))
	return len(b), nil
}

func (u *PushListener) Start(ctx context.Context, notify func(event InstallEvent)) {
	u.currentSize = 0

	refresh := func(finish bool) {
		f := float64(u.currentSize) * float64((time.Second.Milliseconds())/DefRefrashRate.Milliseconds())
		percent := float64(u.lastTotalSize) / float64(u.overallSize)
		if finish {
			percent = 1
		}

		notify(InstallEvent{
			Stage:   InstallByPushDir,
			Current: int64(float64(u.ipaFileSize) * percent),
			Total:   int64(u.ipaFileSize),
			Speed:   int64(f),
			Percent: int(percent * 100),
		})
		u.currentSize = 0
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