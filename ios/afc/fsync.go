package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
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

type statInfo struct {
	stSize       int64
	stBlocks     int64
	stCtime      int64
	stMtime      int64
	stNlink      string
	stIfmt       string
	stLinktarget string
}

func (s *statInfo) isDir() bool {
	return s.stIfmt == "S_IFDIR"
}

func (s *statInfo) isLink() bool {
	return s.stIfmt == "S_IFLNK"
}

func New(device ios.DeviceEntry) (*Connection, error) {
	deviceConn, err := ios.ConnectToService(device, serviceName)
	if err != nil {
		return &Connection{}, err
	}
	return &Connection{deviceConn: deviceConn}, nil
}

func (conn *Connection) sendAfcPacketAndAwaitResponse(packet AfcPacket) (AfcPacket, error) {
	err := Encode(packet, conn.deviceConn.Writer())
	if err != nil {
		return AfcPacket{}, err
	}
	return Decode(conn.deviceConn.Reader())
}

func (conn *Connection) checkOperationStatus(status uint64) bool {
	if status == Afc_operation_status || status == Afc_operation_data || status == Afc_operation_file_close || status == Afc_operation_file_open_result {
		return true
	}
	return false
}

func (conn *Connection) Remove(path string) error {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_remove_path, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return nil
}

func (conn *Connection) MakeDir(path string) error {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_make_dir, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return nil
}

func (conn *Connection) stat(path string) (*statInfo, error) {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_file_info, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return &statInfo{}, err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return &statInfo{}, fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	ret := bytes.Split(response.Payload, []byte{0})
	retLen := len(ret)
	if retLen%2 != 0 {
		retLen = retLen - 1
	}
	statInfoMap := make(map[string]string)
	for i := 0; i <= retLen-2; i = i + 2 {
		k := string(ret[i])
		v := string(ret[i+1])
		statInfoMap[k] = v
	}

	var si statInfo
	si.stSize, _ = strconv.ParseInt(statInfoMap["st_size"], 10, 64)
	si.stBlocks, _ = strconv.ParseInt(statInfoMap["st_blocks"], 10, 64)
	si.stCtime, _ = strconv.ParseInt(statInfoMap["st_birthtime"], 10, 64)
	si.stMtime, _ = strconv.ParseInt(statInfoMap["st_mtime"], 10, 64)
	si.stNlink = statInfoMap["st_nlink"]
	si.stIfmt = statInfoMap["st_ifmt"]
	si.stLinktarget = statInfoMap["st_linktarget"]
	return &si, nil
}

func (conn *Connection) listDir(path string) ([]string, error) {
	headerPayload := []byte(path)
	headerLength := uint64(len(headerPayload))
	thisLength := Afc_header_size + headerLength

	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_read_dir, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return nil, err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return nil, fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	ret := bytes.Split(response.Payload, []byte{0})
	var fileList []string
	for _, v := range ret {
		if string(v) != "." && string(v) != ".." && string(v) != "" {
			fileList = append(fileList, string(v))
		}
	}
	return fileList, nil
}

func (conn *Connection) TreeView(dpath string, prefix string, treePoint bool) error {
	fileInfo, err := conn.stat(dpath)
	if err != nil {
		return err
	}
	namePrefix := "`--"
	if !treePoint {
		namePrefix = "|--"
	}
	tPrefix := prefix + namePrefix
	if fileInfo.isDir() {
		fmt.Printf("%s %s/\n", tPrefix, filepath.Base(dpath))
		fileList, err := conn.listDir(dpath)
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
	} else {
		fmt.Printf("%s %s\n", tPrefix, filepath.Base(dpath))
	}
	return nil
}

func (conn *Connection) openFile(path string, mode uint64) (byte, error) {
	pathBytes := []byte(path)
	headerLength := 8 + uint64(len(pathBytes))
	headerPayload := make([]byte, headerLength)
	binary.LittleEndian.PutUint64(headerPayload, mode)
	copy(headerPayload[8:], pathBytes)
	thisLength := Afc_header_size + headerLength
	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_file_open, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}

	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return 0, err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return 0, fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return response.HeaderPayload[0], nil
}

func (conn *Connection) closeFile(handle byte) error {
	headerPayload := make([]byte, 8)
	headerPayload[0] = handle
	thisLength := 8 + Afc_header_size
	header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_file_close, This_length: thisLength, Entire_length: thisLength}
	conn.packageNumber++
	packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
	response, err := conn.sendAfcPacketAndAwaitResponse(packet)
	if err != nil {
		return err
	}
	if !conn.checkOperationStatus(response.Header.Operation) {
		return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
	}
	return nil
}

func (conn *Connection) pullSingleFile(srcPath, dstPath string) error {
	fileInfo, err := conn.stat(srcPath)
	if err != nil {
		return err
	}
	if fileInfo.isLink() {
		srcPath = fileInfo.stLinktarget
	}
	fd, err := conn.openFile(srcPath, Afc_Mode_RDONLY)
	if err != nil {
		return err
	}
	defer conn.closeFile(fd)

	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	leftSize := fileInfo.stSize
	maxReadSize := 64 * 1024
	for leftSize > 0 {
		headerPayload := make([]byte, 16)
		headerPayload[0] = fd
		thisLength := Afc_header_size + 16
		binary.LittleEndian.PutUint64(headerPayload[8:], uint64(maxReadSize))
		header := AfcPacketHeader{Magic: Afc_magic, Packet_num: conn.packageNumber, Operation: Afc_operation_file_read, This_length: thisLength, Entire_length: thisLength}
		conn.packageNumber++
		packet := AfcPacket{Header: header, HeaderPayload: headerPayload, Payload: make([]byte, 0)}
		response, err := conn.sendAfcPacketAndAwaitResponse(packet)
		if err != nil {
			return err
		}
		if !conn.checkOperationStatus(response.Header.Operation) {
			return fmt.Errorf("Unexpected afc response, expected %x received %x", Afc_operation_status, response.Header.Operation)
		}
		leftSize = leftSize - int64(len(response.Payload))
		f.Write(response.Payload)
	}
	return nil
}

func (conn *Connection) Pull(srcPath, dstPath string) error {
	fileInfo, err := conn.stat(srcPath)
	if err != nil {
		return err
	}
	if fileInfo.isDir() {
		ret, _ := ios.PathExists(dstPath)
		if !ret {
			err = os.MkdirAll(dstPath, os.ModePerm)
			if err != nil {
				return err
			}
		}
		fileList, err := conn.listDir(srcPath)
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
	} else {
		return conn.pullSingleFile(srcPath, dstPath)
	}
	return nil
}

func (conn *Connection) Close() {
	conn.deviceConn.Close()
}
