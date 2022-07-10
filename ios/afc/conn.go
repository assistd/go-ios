package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

const serviceName = "com.apple.afc"

type Connection struct {
	deviceConn    ios.DeviceConnectionInterface
	packageNumber uint64
}

func New(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, serviceName)
	if err != nil {
		return nil, err
	}
	return &Connection{deviceConn: deviceConn}, nil
}

//NewFromConn allows to use AFC on a DeviceConnectionInterface, see crashreport for an example
func NewFromConn(deviceConn ios.DeviceConnectionInterface) *Connection {
	return &Connection{deviceConn: deviceConn}
}

func (conn *Connection) sendAfcPacketAndAwaitResponse(packet AfcPacket) (AfcPacket, error) {
	err := Encode(packet, conn.deviceConn.Writer())
	if err != nil {
		return AfcPacket{}, err
	}
	return Decode(conn.deviceConn.Reader())
}

func (conn *Connection) checkOperationStatus(packet AfcPacket) error {
	if packet.Header.Operation == Afc_operation_status {
		errorCode := binary.LittleEndian.Uint64(packet.HeaderPayload)
		if errorCode != Afc_Err_Success {
			return getError(errorCode)
		}
	}
	return nil
}

func (conn *Connection) request(ops uint64, data, payload []byte) (*AfcPacket, error) {
	header := AfcPacketHeader{
		Magic:         Afc_magic,
		Packet_num:    conn.packageNumber,
		Operation:     ops,
		This_length:   Afc_header_size + uint64(len(data)),
		Entire_length: Afc_header_size + uint64(len(data)+len(payload)),
	}

	packet := AfcPacket{
		Header:        header,
		HeaderPayload: data,
		Payload:       payload,
	}

	conn.packageNumber++
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return nil, err
	}
	if err = conn.checkOperationStatus(response); err != nil {
		return nil, fmt.Errorf("request: unexpected afc status: %v", err)
	}
	return &response, nil
}

func (conn *Connection) Remove(path string) error {
	_, err := conn.request(Afc_operation_remove_path, []byte(path), nil)
	return err
}

func (conn *Connection) Mkdir(path string) error {
	_, err := conn.request(Afc_operation_make_dir, []byte(path), nil)
	return err
}

func (conn *Connection) Stat(path string) (*statInfo, error) {
	response, err := conn.request(Afc_operation_file_info, []byte(path), make([]byte, 0))
	if err != nil {
		return nil, err
	}

	ret := bytes.Split(bytes.TrimSuffix(response.Payload, []byte{0}), []byte{0})
	if len(ret)%2 != 0 {
		log.Fatalf("invalid response: %v %% 2 != 0", len(ret))
	}

	statInfoMap := make(map[string]string)
	for i := 0; i < len(ret); i = i + 2 {
		k := string(ret[i])
		v := string(ret[i+1])
		statInfoMap[k] = v
	}

	var si statInfo
	si.name = filepath.Base(path)
	si.stSize, _ = strconv.ParseInt(statInfoMap["st_size"], 10, 64)
	si.stBlocks, _ = strconv.ParseInt(statInfoMap["st_blocks"], 10, 64)
	si.stCtime, _ = strconv.ParseInt(statInfoMap["st_birthtime"], 10, 64)
	si.stMtime, _ = strconv.ParseInt(statInfoMap["st_mtime"], 10, 64)
	si.stNlink = statInfoMap["st_nlink"]
	si.stIfmt = statInfoMap["st_ifmt"]
	si.stLinktarget = statInfoMap["st_linktarget"]
	return &si, nil
}

func (conn *Connection) ListDir(path string) ([]string, error) {
	response, err := conn.request(Afc_operation_read_dir, []byte(path), nil)
	if err != nil {
		return nil, err
	}

	ret := bytes.Split(bytes.TrimSuffix(response.Payload, []byte{0}), []byte{0})
	var fileList []string
	for _, v := range ret {
		if string(v) != "." && string(v) != ".." && string(v) != "" {
			fileList = append(fileList, string(v))
		}
	}
	return fileList, nil
}

//ListFiles returns all files in the given directory, matching the pattern.
//Example: ListFiles(".", "*") returns all files and dirs in the current path the afc connection is in
func (conn *Connection) ListFiles(cwd string, matchPattern string) ([]string, error) {
	files, err := conn.ListDir(cwd)
	if err != nil {
		return nil, err
	}

	var filteredFiles []string
	for _, f := range files {
		if f == "" {
			continue
		}
		matches, err := filepath.Match(matchPattern, f)
		if err != nil {
			log.Warn("error while matching pattern", err)
		}
		if matches {
			filteredFiles = append(filteredFiles, f)
		}
	}
	return filteredFiles, nil
}

func (conn *Connection) TreeView(dpath string, prefix string, treePoint bool) error {
	fileInfo, err := conn.Stat(dpath)
	if err != nil {
		return err
	}

	namePrefix := "`--"
	if !treePoint {
		namePrefix = "|--"
	}
	tPrefix := prefix + namePrefix
	if !fileInfo.IsDir() {
		//return fmt.Errorf("error: %v is not dir", dpath)
		fmt.Printf("%s %s\n", tPrefix, filepath.Base(dpath))
		return nil
	}

	fmt.Printf("%s %s/\n", tPrefix, filepath.Base(dpath))
	fileList, err := conn.ListDir(dpath)
	if err != nil {
		return err
	}
	for i, v := range fileList {
		tp := false
		if i == len(fileList)-1 {
			tp = true
		}
		rp := prefix + "    "
		if !treePoint {
			rp = prefix + "|   "
		}
		nPath := path.Join(dpath, v)
		err = conn.TreeView(nPath, rp, tp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (conn *Connection) OpenFile(path string, mode uint64) (uint64, error) {
	data := make([]byte, len(path)+8)
	binary.LittleEndian.PutUint64(data, mode)
	copy(data[8:], path)
	response, err := conn.request(Afc_operation_file_open, data, nil)
	if err != nil {
		return 0, err
	}
	fd := binary.LittleEndian.Uint64(response.HeaderPayload)
	if fd == 0 {
		return 0, fmt.Errorf("file descriptor should not be zero")
	}

	return fd, nil
}

func (conn *Connection) CloseFile(fd uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)
	_, err := conn.request(Afc_operation_file_close, data, nil)
	return err
}

func (conn *Connection) PullFile(srcPath, dstPath string) error {
	fileInfo, err := conn.Stat(srcPath)
	if err != nil {
		return err
	}

	if fileInfo.IsLink() {
		srcPath = fileInfo.stLinktarget
	}
	fd, err := conn.OpenFile(srcPath, Afc_Mode_RDONLY)
	if err != nil {
		return err
	}
	defer conn.CloseFile(fd)

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	leftSize := fileInfo.stSize
	maxReadSize := 64 * 1024
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(maxReadSize))
	for leftSize > 0 {
		response, err := conn.request(Afc_operation_file_read, data, nil)
		if err != nil {
			return err
		}
		leftSize -= int64(len(response.Payload))
		f.Write(response.Payload)
	}
	return nil
}

func (conn *Connection) Pull(srcPath, dstPath string) error {
	fileInfo, err := conn.Stat(srcPath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return conn.PullFile(srcPath, dstPath)
	}
	ret, _ := ios.PathExists(dstPath)
	if !ret {
		err = os.MkdirAll(dstPath, 0755)
		if err != nil {
			return err
		}
	}
	fileList, err := conn.ListDir(srcPath)
	if err != nil {
		return err
	}
	for _, v := range fileList {
		sp := path.Join(srcPath, v)
		dp := path.Join(dstPath, v)
		err = conn.Pull(sp, dp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (conn *Connection) Push(srcPath, dstPath string) error {
	ret, _ := ios.PathExists(srcPath)
	if !ret {
		return fmt.Errorf("%s: no such file", srcPath)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if fileInfo, _ := conn.Stat(dstPath); fileInfo != nil {
		if fileInfo.IsDir() {
			dstPath = path.Join(dstPath, filepath.Base(srcPath))
		}
	}

	fd, err := conn.OpenFile(dstPath, Afc_Mode_WR)
	if err != nil {
		return err
	}
	defer conn.CloseFile(fd)

	maxWriteSize := 64 * 1024
	chunk := make([]byte, maxWriteSize)
	for {
		n, err := f.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, fd)
		_, err = conn.request(Afc_operation_file_write, data, chunk[0:n])
		if err != nil {
			return err
		}
	}
	return nil
}

func (conn *Connection) Close() {
	conn.deviceConn.Close()
}
