package afc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"strconv"
	"time"
)

const serviceName = "com.apple.afc"

type Connection struct {
	deviceConn    ios.DeviceConnectionInterface
	packageNumber uint64
}

func NewConn(device ios.DeviceEntry) (*Connection, error) {
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

func (conn *Connection) RemovePath(path string) error {
	_, err := conn.request(Afc_operation_remove_path, []byte(path), nil)
	return err
}

func (conn *Connection) RenamePath(from, to string) error {
	data := make([]byte, len(from)+1+len(to)+1)
	copy(data, from)
	copy(data[len(from)+1:], to)
	_, err := conn.request(Afc_operation_rename_path, data, nil)
	return err
}

func (conn *Connection) MakeDir(path string) error {
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

func (conn *Connection) ReadDir(path string) ([]string, error) {
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

func (conn *Connection) ReadFile(fd uint64, p []byte) (n int, err error) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(len(p)))
	response, err := conn.request(Afc_operation_file_read, data, nil)
	if err != nil {
		return 0, err
	}

	log.Info("inbuf len:%v, read len:%v", len(p), len(response.Payload))
	if len(response.Payload) > len(p) {
		log.Fatalf("inbuf len:%v, read len:%v", len(p), len(response.Payload))
	}

	copy(p, response.Payload)
	return len(response.Payload), nil
}

func (conn *Connection) WriteFile(fd uint64, p []byte) (n int, err error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)
	_, err = conn.request(Afc_operation_file_write, data, p)
	return len(p), err
}

func (conn *Connection) CloseFile(fd uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)
	_, err := conn.request(Afc_operation_file_close, data, nil)
	return err
}

func (conn *Connection) LockFile(fd uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)
	_, err := conn.request(Afc_operation_file_close, data, nil)
	return err
}

func (conn *Connection) SeekFile(fd uint64, offset int64, whence int) (int64, error) {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(whence))
	binary.LittleEndian.PutUint64(data[16:], uint64(offset))
	response, err := conn.request(Afc_operation_file_seek, data, nil)
	pos := binary.LittleEndian.Uint64(response.HeaderPayload)
	log.Println("seek:", hex.Dump(response.HeaderPayload))
	return int64(pos), err
}

func (conn *Connection) TellFile(fd uint64) (uint64, error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, fd)
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
	_, err := conn.request(Afc_operation_file_set_size, data, nil)
	return err
}

func (conn *Connection) Truncate(path string, size uint64) error {
	data := make([]byte, 8+len(path))
	binary.LittleEndian.PutUint64(data, size)
	copy(data[8:], path)
	_, err := conn.request(Afc_operation_TRUNCATE, data, nil)
	return err
}

func (conn *Connection) MakeLink(link LinkType, target, linkname string) error {
	data := make([]byte, 8+len(target)+1+len(linkname)+1)
	binary.LittleEndian.PutUint64(data, uint64(link))
	copy(data[8:], target)
	copy(data[8+len(target)+1:], linkname)
	_, err := conn.request(Afc_operation_make_link, data, nil)
	return err
}

func (conn *Connection) SetFileTime(path string, t time.Time) error {
	data := make([]byte, 8+len(path)+1)
	binary.LittleEndian.PutUint64(data, uint64(t.UnixNano()))
	copy(data[8:], path)
	_, err := conn.request(Afc_operation_set_file_time, data, nil)
	return err
}

func (conn *Connection) RemovePathAndContents(path string) error {
	_, err := conn.request(AFC_OP_REMOVE_PATH_AND_CONTENTS, []byte(path), nil)
	return err
}

func (conn *Connection) Close() {
	conn.deviceConn.Close()
}
