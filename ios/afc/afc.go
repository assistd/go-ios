package afc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Afc_magic       uint64 = 0x4141504c36414643
	Afc_header_size uint64 = 40

	Afc_operation_status              uint64 = 0x00000001
	Afc_operation_data                uint64 = 0x00000002 // Data
	Afc_operation_read_dir            uint64 = 0x00000003 // ReadDir
	Afc_operation_READ_FILE           uint64 = 0x00000004 // ReadFile
	Afc_operation_WRITE_FILE          uint64 = 0x00000005 // WriteFile
	Afc_operation_WRITE_PART          uint64 = 0x00000006 // WritePart
	Afc_operation_TRUNCATE            uint64 = 0x00000007 // TruncateFile
	Afc_operation_remove_path         uint64 = 0x00000008 // RemovePath
	Afc_operation_make_dir            uint64 = 0x00000009 // MakeDir
	Afc_operation_file_info           uint64 = 0x0000000A // GetFileInfo
	Afc_operation_get_devinfo         uint64 = 0x0000000B // GetDeviceInfo
	Afc_operation_write_file_atom     uint64 = 0x0000000C // WriteFileAtomic (tmp file+rename)
	Afc_operation_file_open           uint64 = 0x0000000D // FileRefOpen
	Afc_operation_file_open_result    uint64 = 0x0000000E // FileRefOpenResult
	Afc_operation_file_read           uint64 = 0x0000000F // FileRefRead
	Afc_operation_file_write          uint64 = 0x00000010 // FileRefWrite
	Afc_operation_file_seek           uint64 = 0x00000011 // FileRefSeek
	Afc_operation_file_tell           uint64 = 0x00000012 // FileRefTell
	Afc_operation_file_tell_result    uint64 = 0x00000013 // FileRefTellResult
	Afc_operation_file_close          uint64 = 0x00000014 // FileRefClose
	Afc_operation_file_set_size       uint64 = 0x00000015 // FileRefSetFileSize(ftruncate)
	Afc_operation_get_con_info        uint64 = 0x00000016 // GetConnectionInfo
	Afc_operation_set_conn_options    uint64 = 0x00000017 // SetConnectionOptions
	Afc_operation_rename_path         uint64 = 0x00000018 // RenamePath
	Afc_operation_set_fs_bs           uint64 = 0x00000019 // SetFSBlockSize (0x800000)
	Afc_operation_set_socket_bs       uint64 = 0x0000001A // SetSocketBlockSize
	Afc_operation_file_lock           uint64 = 0x0000001B // FileRefLock
	Afc_operation_make_link           uint64 = 0x0000001C // MakeLink
	Afc_operation_set_file_time       uint64 = 0x0000001E // set st_mtime
	Afc_operation_get_file_Hash_range uint64 = 0x0000001F // GetFileHashWithRange

	/* iOS 6+ */
	AFC_OP_FILE_SET_IMMUTABLE_HINT   = 0x00000020 /* FileRefSetImmutableHint */
	AFC_OP_GET_SIZE_OF_PATH_CONTENTS = 0x00000021 /* GetSizeOfPathContents */
	AFC_OP_REMOVE_PATH_AND_CONTENTS  = 0x00000022 /* RemovePathAndContents */
	AFC_OP_DIR_OPEN                  = 0x00000023 /* DirectoryEnumeratorRefOpen */
	AFC_OP_DIR_OPEN_RESULT           = 0x00000024 /* DirectoryEnumeratorRefOpenResult */
	AFC_OP_DIR_READ                  = 0x00000025 /* DirectoryEnumeratorRefRead */
	AFC_OP_DIR_CLOSE                 = 0x00000026 /* DirectoryEnumeratorRefClose */
	/* iOS 7+ */
	AFC_OP_FILE_READ_OFFSET  = 0x00000027 /* FileRefReadWithOffset */
	AFC_OP_FILE_WRITE_OFFSET = 0x00000028 /* FileRefWriteWithOffset */
)

type LinkType int

const (
	AFC_HARDLINK LinkType = 1
	AFC_SYMLINK  LinkType = 2
)

const (
	Afc_Mode_RDONLY   uint64 = 0x00000001 // r,  O_RDONLY
	Afc_Mode_RW       uint64 = 0x00000002 // r+, O_RDWR   | O_CREAT
	Afc_Mode_WRONLY   uint64 = 0x00000003 // w,  O_WRONLY | O_CREAT  | O_TRUNC
	Afc_Mode_WR       uint64 = 0x00000004 // w+, O_RDWR   | O_CREAT  | O_TRUNC
	Afc_Mode_APPEND   uint64 = 0x00000005 // a,  O_WRONLY | O_APPEND | O_CREAT
	Afc_Mode_RDAPPEND uint64 = 0x00000006 // a+, O_RDWR   | O_APPEND | O_CREAT
)

const (
	Afc_Err_Success                = 0
	Afc_Err_UnknownError           = 1
	Afc_Err_OperationHeaderInvalid = 2
	Afc_Err_NoResources            = 3
	Afc_Err_ReadError              = 4
	Afc_Err_WriteError             = 5
	Afc_Err_UnknownPacketType      = 6
	Afc_Err_InvalidArgument        = 7
	Afc_Err_ObjectNotFound         = 8
	Afc_Err_ObjectIsDir            = 9
	Afc_Err_PermDenied             = 10
	Afc_Err_ServiceNotConnected    = 11
	Afc_Err_OperationTimeout       = 12
	Afc_Err_TooMuchData            = 13
	Afc_Err_EndOfData              = 14
	Afc_Err_OperationNotSupported  = 15
	Afc_Err_ObjectExists           = 16
	Afc_Err_ObjectBusy             = 17
	Afc_Err_NoSpaceLeft            = 18
	Afc_Err_OperationWouldBlock    = 19
	Afc_Err_IoError                = 20
	Afc_Err_OperationInterrupted   = 21
	Afc_Err_OperationInProgress    = 22
	Afc_Err_InternalError          = 23
	Afc_Err_MuxError               = 30
	Afc_Err_NoMemory               = 31
	Afc_Err_NotEnoughData          = 32
	Afc_Err_DirNotEmpty            = 33
)

func getError(errorCode uint64) error {
	switch errorCode {
	case Afc_Err_UnknownError:
		return errors.New("UnknownError")
	case Afc_Err_OperationHeaderInvalid:
		return errors.New("OperationHeaderInvalid")
	case Afc_Err_NoResources:
		return errors.New("NoResources")
	case Afc_Err_ReadError:
		return errors.New("ReadError")
	case Afc_Err_WriteError:
		return errors.New("WriteError")
	case Afc_Err_UnknownPacketType:
		return errors.New("UnknownPacketType")
	case Afc_Err_InvalidArgument:
		return errors.New("InvalidArgument")
	case Afc_Err_ObjectNotFound:
		return errors.New("ObjectNotFound")
	case Afc_Err_ObjectIsDir:
		return errors.New("ObjectIsDir")
	case Afc_Err_PermDenied:
		return errors.New("PermDenied")
	case Afc_Err_ServiceNotConnected:
		return errors.New("ServiceNotConnected")
	case Afc_Err_OperationTimeout:
		return errors.New("OperationTimeout")
	case Afc_Err_TooMuchData:
		return errors.New("TooMuchData")
	case Afc_Err_EndOfData:
		return errors.New("EndOfData")
	case Afc_Err_OperationNotSupported:
		return errors.New("OperationNotSupported")
	case Afc_Err_ObjectExists:
		return errors.New("ObjectExists")
	case Afc_Err_ObjectBusy:
		return errors.New("ObjectBusy")
	case Afc_Err_NoSpaceLeft:
		return errors.New("NoSpaceLeft")
	case Afc_Err_OperationWouldBlock:
		return errors.New("OperationWouldBlock")
	case Afc_Err_IoError:
		return errors.New("IoError")
	case Afc_Err_OperationInterrupted:
		return errors.New("OperationInterrupted")
	case Afc_Err_OperationInProgress:
		return errors.New("OperationInProgress")
	case Afc_Err_InternalError:
		return errors.New("InternalError")
	case Afc_Err_MuxError:
		return errors.New("MuxError")
	case Afc_Err_NoMemory:
		return errors.New("NoMemory")
	case Afc_Err_NotEnoughData:
		return errors.New("NotEnoughData")
	case Afc_Err_DirNotEmpty:
		return errors.New("DirNotEmpty")
	default:
		return nil
	}
}

type AfcPacketHeader struct {
	Magic         uint64
	Entire_length uint64
	This_length   uint64
	Packet_num    uint64
	Operation     uint64
}

type AfcPacket struct {
	Header        AfcPacketHeader
	HeaderPayload []byte
	Payload       []byte
}

func Decode(reader io.Reader) (AfcPacket, error) {
	var header AfcPacketHeader
	err := binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		return AfcPacket{}, err
	}
	if header.Magic != Afc_magic {
		return AfcPacket{}, fmt.Errorf("Wrong magic:%x expected: %x", header.Magic, Afc_magic)
	}
	headerPayloadLength := header.This_length - Afc_header_size
	headerPayload := make([]byte, headerPayloadLength)
	_, err = io.ReadFull(reader, headerPayload)
	if err != nil {
		return AfcPacket{}, err
	}
	contentPayloadLength := header.Entire_length - header.This_length
	payload := make([]byte, contentPayloadLength)
	_, err = io.ReadFull(reader, payload)
	if err != nil {
		return AfcPacket{}, err
	}
	return AfcPacket{header, headerPayload, payload}, nil
}

func Encode(packet AfcPacket, writer io.Writer) error {
	err := binary.Write(writer, binary.LittleEndian, packet.Header)
	if err != nil {
		return err
	}
	_, err = writer.Write(packet.HeaderPayload)
	if err != nil {
		return err
	}

	_, err = writer.Write(packet.Payload)
	if err != nil {
		return err
	}
	return nil
}
