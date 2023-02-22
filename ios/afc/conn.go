package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

const afcServiceName = "com.apple.afc"

type Connection struct {
	deviceConn    ios.DeviceConnectionInterface
	packageNumber uint64
	mutex         sync.Mutex
}

func NewAfcConn(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, afcServiceName)
	if err != nil {
		return nil, err
	}
	return &Connection{deviceConn: deviceConn}, nil
}

// NewFromConn allows to use AFC on a DeviceConnectionInterface, see crashreport for an example
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
		return nil, fmt.Errorf("unexpected afc status: %v", err)
	}
	return &response, nil
}

func (conn *Connection) RemovePath(path string) error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_remove_path, []byte(path), nil)
	return err
}

func (conn *Connection) RenamePath(from, to string) error {
	data := make([]byte, len(from)+1+len(to)+1)
	copy(data, from)
	copy(data[len(from)+1:], to)
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_rename_path, data, nil)
	return err
}

func (conn *Connection) MakeDir(path string) error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_make_dir, []byte(path), nil)
	return err
}

func (conn *Connection) Stat(path string) (*StatInfo, error) {
	conn.mutex.Lock()
	response, err := conn.request(Afc_operation_file_info, []byte(path), nil)
	if err != nil {
		conn.mutex.Unlock()
		return nil, fmt.Errorf("cannot stat '%v': %v", path, err)
	}
	conn.mutex.Unlock()

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

	var si StatInfo
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

func (conn *Connection) ReadDir(path string) ([]string, error) {
	// log.Infof("ReadDir path:%v", path)
	conn.mutex.Lock()
	response, err := conn.request(Afc_operation_read_dir, []byte(path), nil)
	if err != nil {
		conn.mutex.Unlock()
		log.Infof("ReadDir error:%v", err)
		return nil, err
	}
	conn.mutex.Unlock()

	ret := bytes.Split(bytes.TrimSuffix(response.Payload, []byte{0}), []byte{0})
	var fileList []string
	for _, v := range ret {
		if string(v) != "." && string(v) != ".." && string(v) != "" {
			fileList = append(fileList, string(v))
		}
	}

	// log.Infof("ReadDir end:%v", fileList)
	return fileList, nil
}

func (conn *Connection) OpenFile(path string, mode uint64) (uint64, error) {
	// log.Infof("OpenFile path:%v", path)
	data := make([]byte, 8+len(path)+1)
	binary.LittleEndian.PutUint64(data, mode)
	copy(data[8:], path)
	conn.mutex.Lock()
	response, err := conn.request(Afc_operation_file_open, data, make([]byte, 0))
	if err != nil {
		conn.mutex.Unlock()
		log.Errorf("OpenFile path:%v err:%v", path, err)
		return 0, err
	}
	conn.mutex.Unlock()

	fd := binary.LittleEndian.Uint64(response.HeaderPayload)
	if fd == 0 {
		return 0, fmt.Errorf("file descriptor should not be zero")
	}

	return fd, nil
}

func (conn *Connection) ReadFile(fd uint64, p []byte) (n int, err error) {
	// log.Infof("ReadFile inbuf pd:%v, read len:%v", fd, len(p))
	// defer log.Info("ReadFile end")
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(len(p)))

	conn.mutex.Lock()
	response, err := conn.request(Afc_operation_file_read, data, nil)
	if err != nil {
		conn.mutex.Unlock()
		return 0, err
	}
	conn.mutex.Unlock()

	// log.Infof("inbuf len:%v, read len:%v", len(p), len(response.Payload))
	n = len(response.Payload)
	if n > len(p) {
		log.Fatalf("inbuf len:%v, read len:%v", len(p), len(response.Payload))
	}
	if n == 0 {
		return n, io.EOF
	}

	if n < len(p) {
		err = io.EOF
	}
	copy(p, response.Payload)
	return
}

func (conn *Connection) WriteFile(fd uint64, p []byte) (n int, err error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err = conn.request(Afc_operation_file_write, data, p)
	return len(p), err
}

func (conn *Connection) CloseFile(fd uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_file_close, data, nil)
	return err
}

func (conn *Connection) LockFile(fd uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_file_close, data, nil)
	return err
}

// SeekFile whence is SEEK_SET, SEEK_CUR, or SEEK_END.
func (conn *Connection) SeekFile(fd uint64, offset int64, whence int) (int64, error) {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(whence))
	binary.LittleEndian.PutUint64(data[16:], uint64(offset))

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_file_seek, data, nil)
	if err != nil {
		return 0, err
	}

	data2 := make([]byte, 8)
	binary.LittleEndian.PutUint64(data2, fd)
	response, err := conn.request(Afc_operation_file_tell, data2, nil)
	if err != nil {
		return 0, err
	}

	pos := binary.LittleEndian.Uint64(response.HeaderPayload)
	return int64(pos), nil
}

func (conn *Connection) TellFile(fd uint64) (uint64, error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	response, err := conn.request(Afc_operation_file_tell, data, nil)
	if err != nil {
		return 0, err
	}

	pos := binary.LittleEndian.Uint64(response.HeaderPayload)
	return pos, err
}

func (conn *Connection) TruncateFile(fd uint64, size int64) error {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data, uint64(size))

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_file_set_size, data, nil)
	return err
}

func (conn *Connection) Truncate(path string, size uint64) error {
	data := make([]byte, 8+len(path))
	binary.LittleEndian.PutUint64(data, size)
	copy(data[8:], path)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_TRUNCATE, data, nil)
	return err
}

func (conn *Connection) MakeLink(link LinkType, target, linkname string) error {
	data := make([]byte, 8+len(target)+1+len(linkname)+1)
	binary.LittleEndian.PutUint64(data, uint64(link))
	copy(data[8:], target)
	copy(data[8+len(target)+1:], linkname)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_make_link, data, nil)
	return err
}

func (conn *Connection) SetFileTime(path string, t time.Time) error {
	data := make([]byte, 8+len(path)+1)
	binary.LittleEndian.PutUint64(data, uint64(t.UnixNano()))
	copy(data[8:], path)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(Afc_operation_set_file_time, data, nil)
	return err
}

func (conn *Connection) RemovePathAndContents(path string) error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	_, err := conn.request(AFC_OP_REMOVE_PATH_AND_CONTENTS, []byte(path), nil)
	return err
}

func (conn *Connection) Close() {
	log.Infof("Close connection")
	conn.deviceConn.Close()
}
