package virtualfs

import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/spf13/afero"
	"os"
	"strings"
	"syscall"
	"time"
)

type SandBoxFs struct {
	DeviceFs
	bundleId string
}

func NewSandBoxFs(udid string, bundleId string, mountPath string) *SandBoxFs {
	return &SandBoxFs{
		DeviceFs: DeviceFs{
			udid:      udid,
			conn:      nil,
			mountPath: mountPath,
		},
		bundleId: bundleId}
}

func (fs *SandBoxFs) initialize() error {
	if fs.conn == nil {
		deviceEntry, err := ios.GetDevice(fs.udid)
		if err != nil {
			return err
		}
		houseArrestConn, err := afc.NewHouseArrestConn(deviceEntry, fs.bundleId)
		if err != nil {
			return err
		}
		fs.conn = houseArrestConn
	}
	return nil
}

func (fs *SandBoxFs) getDevicePath(absPath string) string {
	trimmedPath := strings.TrimPrefix(absPath, fs.mountPath)
	if !strings.HasPrefix(trimmedPath, "/") {
		trimmedPath = "/" + trimmedPath
	}
	return trimmedPath
}

func (fs *SandBoxFs) Create(name string) (afero.File, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	name = fs.getDevicePath(name)

	fd, err := fs.conn.OpenFile(name, afc.Afc_Mode_WR) // O_RDWR | O_CREAT | O_TRUNC
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	return afc.NewFile(fs.conn, fd, name, false), nil
}

func (fs *SandBoxFs) Mkdir(name string, perm os.FileMode) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	name = fs.getDevicePath(name)

	err := fs.conn.MakeDir(name)
	return err
}

func (fs *SandBoxFs) MkdirAll(path string, perm os.FileMode) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	path = fs.getDevicePath(path)

	err := fs.conn.MakeDir(path)
	return err
}

func (fs *SandBoxFs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *SandBoxFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	name = fs.getDevicePath(name)

	info, err := fs.conn.Stat(name)
	if err == nil {
		if info.IsDir() {
			return afc.NewFile(fs.conn, 0, name, true), nil
		}
	}

	var afcFlags uint64
	switch flag & 0x03 {
	case os.O_RDONLY:
		afcFlags = afc.Afc_Mode_RDONLY
	case os.O_WRONLY:
		{
			if flag&os.O_TRUNC != 0 {
				afcFlags = afc.Afc_Mode_WRONLY
			} else if flag&os.O_APPEND != 0 {
				afcFlags = afc.Afc_Mode_APPEND
			} else {
				afcFlags = afc.Afc_Mode_RW
			}
		}
	case os.O_RDWR:
		{
			if flag&os.O_TRUNC != 0 {
				afcFlags = afc.Afc_Mode_WR
			} else if flag&os.O_APPEND != 0 {
				afcFlags = afc.Afc_Mode_RDAPPEND
			} else {
				afcFlags = afc.Afc_Mode_RW
			}
		}
	default:
		return nil, fmt.Errorf("invalid flag")
	}

	fd, err := fs.conn.OpenFile(name, afcFlags)
	if err != nil {
		return nil, err
	}

	return afc.NewFile(fs.conn, fd, name, false), nil
}

func (fs *SandBoxFs) Remove(name string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	name = fs.getDevicePath(name)

	return fs.conn.RemovePath(name)
}

func (fs *SandBoxFs) RemoveAll(path string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	path = fs.getDevicePath(path)

	return fs.conn.RemovePathAndContents(path)
}

func (fs *SandBoxFs) Rename(oldname, newname string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	oldname = fs.getDevicePath(oldname)
	newname = fs.getDevicePath(newname)

	return fs.conn.RenamePath(oldname, newname)
}

func (fs *SandBoxFs) Stat(name string) (os.FileInfo, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	name = fs.getDevicePath(name)

	return fs.conn.Stat(name)
}

func (fs *SandBoxFs) Name() string { return "iosvirtualfs" }

func (fs *SandBoxFs) Chmod(name string, mode os.FileMode) error {
	return syscall.EPERM
}

func (fs *SandBoxFs) Chown(name string, uid, gid int) error {
	return syscall.EPERM
}

func (fs *SandBoxFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return syscall.EPERM
}
