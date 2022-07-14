package virtualfs

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/spf13/afero"
	"os"
	"path"
	"time"
)

type VirtualRootFs struct {
	device      ios.DeviceEntry
	mountPoints map[string]VirtualFs
}

func (fs *VirtualRootFs) findMountPoint(name string) afero.Fs {
	for {
		if val, ok := fs.mountPoints[name]; ok {
			return val
		}
		if name == "/" {
			break
		}
		name = path.Dir(name)
	}
	return nil
}

func (fs *VirtualRootFs) newVirtualFile(name string) *VirtualFile {
	return &VirtualFile{
		vfs:     fs,
		absPath: name,
		isDir:   true,
	}
}

func (fs *VirtualRootFs) Create(name string) (afero.File, error) {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Create(name)
	}
	return nil, nil
}

func (fs *VirtualRootFs) Mkdir(name string, perm os.FileMode) error {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Mkdir(name, perm)
	}
	return nil
}

func (fs *VirtualRootFs) MkdirAll(path string, perm os.FileMode) error {
	mp := fs.findMountPoint(path)
	if mp != nil {
		return mp.MkdirAll(path, perm)
	}
	return nil
}

func (fs *VirtualRootFs) Open(name string) (afero.File, error) {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Open(name)
	}
	return fs.newVirtualFile(name), nil
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *VirtualRootFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.OpenFile(name, flag, perm)
	} else {
		return fs.newVirtualFile(name), nil
	}
}

func (fs *VirtualRootFs) Remove(name string) error {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Remove(name)
	}
	return nil
}

func (fs *VirtualRootFs) RemoveAll(path string) error {
	mp := fs.findMountPoint(path)
	if mp != nil {
		return mp.RemoveAll(path)
	}
	return nil
}

func (fs *VirtualRootFs) Rename(oldname, newname string) error {
	mp := fs.findMountPoint(oldname)
	if mp != nil {
		return mp.Rename(oldname, newname)
	}
	return nil
}

func (fs *VirtualRootFs) Stat(name string) (os.FileInfo, error) {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Stat(name)
	} else {
		return newDirStat(path.Base(name)), nil
	}
}

func (fs *VirtualRootFs) Name() string { return "iOSVirtualRootFs" }

func (fs *VirtualRootFs) Chmod(name string, mode os.FileMode) error {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Chmod(name, mode)
	}
	return nil
}

func (fs *VirtualRootFs) Chown(name string, uid, gid int) error {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Chown(name, uid, gid)
	}
	return nil
}

func (fs *VirtualRootFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Chtimes(name, atime, mtime)
	}
	return nil
}

func (fs *VirtualRootFs) DoMount(mountPath string, vfs VirtualFs) {
	fs.mountPoints[mountPath] = vfs
}

func (fs *VirtualRootFs) MountPoints() map[string]VirtualFs {
	return fs.mountPoints
}

func (fs *VirtualRootFs) ReadDir(absPath string) (fi []os.FileInfo, err error) {
	for mountPath, _ := range fs.MountPoints() {
		if path.Dir(mountPath) == absPath {
			fileInfo := newDirStat(path.Base(mountPath))
			fi = append(fi, fileInfo)
		}
	}
	return fi, nil
}
