package virtualfs

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/spf13/afero"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

type VirtualSandboxGroupFs struct {
	udid        string
	mountPath   string
	sandboxesFs map[string]VirtualFs
	initialized bool
}

func NewVirtualSandboxGroupFs(udid string, mountPath string) VirtualFs {
	return &VirtualSandboxGroupFs{
		udid:        udid,
		mountPath:   mountPath,
		sandboxesFs: make(map[string]VirtualFs),
		initialized: false,
	}
}

func (fs *VirtualSandboxGroupFs) initialize() error {
	if fs.initialized {
		return nil
	}
	fs.initialized = true
	deviceEntry, err := ios.GetDevice(fs.udid)
	if err != nil {
		return err
	}
	instproxy, err := installationproxy.New(deviceEntry)
	if err != nil {
		return err
	}
	appInfo, err := instproxy.BrowseAnyApps()
	if err != nil {
		return err
	}
	for _, app := range appInfo {
		if app.UIFileSharingEnabled {
			mountPath := path.Join(fs.mountPath, app.CFBundleIdentifier)
			sandboxFS := NewSandBoxFs(fs.udid, app.CFBundleIdentifier, mountPath)

			fs.DoMount(mountPath, sandboxFS)
		}
	}
	instproxy.Close()
	return nil
}

func (fs *VirtualSandboxGroupFs) newVirtualFile(name string) *VirtualFile {
	return &VirtualFile{
		vfs:     fs,
		absPath: name,
		isDir:   true,
	}
}

func (fs *VirtualSandboxGroupFs) findMountPoint(name string) VirtualFs {
	for mountPath, vfs := range fs.MountPoints() {
		if strings.HasPrefix(name, mountPath) {
			trimmedName := strings.TrimPrefix(name, mountPath)
			if trimmedName == "" || strings.HasPrefix(trimmedName, "/") {
				return vfs
			}
		}
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) Create(name string) (afero.File, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Create(name)
	}
	return nil, nil
}

func (fs *VirtualSandboxGroupFs) Mkdir(name string, perm os.FileMode) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Mkdir(name, perm)
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) MkdirAll(path string, perm os.FileMode) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	mp := fs.findMountPoint(path)
	if mp != nil {
		return mp.MkdirAll(path, perm)
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *VirtualSandboxGroupFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.OpenFile(name, flag, perm)
	}
	return fs.newVirtualFile(name), nil
}

func (fs *VirtualSandboxGroupFs) Remove(name string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	mp := fs.findMountPoint(name)
	if mp != nil {
		return mp.Remove(name)
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) RemoveAll(path string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	mp := fs.findMountPoint(path)
	if mp != nil {
		return mp.RemoveAll(path)
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) Rename(oldname, newname string) error {
	if err := fs.initialize(); err != nil {
		return err
	}
	mp := fs.findMountPoint(oldname)
	if mp != nil {
		return mp.Rename(oldname, newname)
	}
	return nil
}

func (fs *VirtualSandboxGroupFs) Stat(name string) (os.FileInfo, error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}

	vfs := fs.findMountPoint(name)
	if vfs != nil {
		return vfs.Stat(name)
	}
	return newDirStat(path.Base(name)), nil
}

func (fs *VirtualSandboxGroupFs) Name() string { return "iOSVirtualSandboxesFs" }

func (fs *VirtualSandboxGroupFs) Chmod(name string, mode os.FileMode) error {
	return syscall.EPERM
}

func (fs *VirtualSandboxGroupFs) Chown(name string, uid, gid int) error {
	return syscall.EPERM
}

func (fs *VirtualSandboxGroupFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return syscall.EPERM
}

func (fs *VirtualSandboxGroupFs) DoMount(mountPath string, vfs VirtualFs) {
	fs.sandboxesFs[mountPath] = vfs
}

func (fs *VirtualSandboxGroupFs) MountPoints() map[string]VirtualFs {
	return fs.sandboxesFs
}

func (fs *VirtualSandboxGroupFs) ReadDir(absPath string) (fi []os.FileInfo, err error) {
	if err := fs.initialize(); err != nil {
		return nil, err
	}
	for mountPath, _ := range fs.MountPoints() {
		if path.Dir(mountPath) == absPath {
			fileInfo := newDirStat(path.Base(mountPath))
			fi = append(fi, fileInfo)
		}
	}
	return fi, nil
}
