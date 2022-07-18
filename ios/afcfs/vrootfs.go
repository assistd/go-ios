package afcfs

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

const (
	afcMountPath     = "/afc"
	sandboxMountPath = "/apps"
)

type VirtualRootFs struct {
	afero.Fs
	device      ios.DeviceEntry
	mountPoints map[string]*afc.Fsync
}

func NewVfs(device ios.DeviceEntry) (*VirtualRootFs, error) {
	afcFs, err := afc.New(device)
	if err != nil {
		return nil, err
	}

	rootFs := &VirtualRootFs{
		device:      device,
		mountPoints: make(map[string]*afc.Fsync),
	}
	rootFs.Mount(afcMountPath, afcFs)
	_ = rootFs.mountAppsSandbox()
	return rootFs, nil
}

func (fs *VirtualRootFs) mountAppsSandbox() error {
	proxy, err := installationproxy.New(fs.device)
	if err != nil {
		return err
	}
	defer proxy.Close()

	appInfo, err := proxy.BrowseUserApps()
	if err != nil {
		return err
	}

	for _, app := range appInfo {
		if !app.UIFileSharingEnabled {
			continue
		}
		sandboxFs, err := afc.New2(fs.device, app.CFBundleIdentifier)
		if err != nil {
			log.Errorf("mount %v error:%v", app.CFBundleIdentifier, err)
			continue
		}

		log.Infoln("mount", app.CFBundleIdentifier)
		fs.Mount(path.Join(sandboxMountPath, app.CFBundleIdentifier), sandboxFs)
	}

	return nil
}

func (fs *VirtualRootFs) trimPath(path string, mountPoint string) string {
	trimmedPath := strings.TrimPrefix(path, mountPoint)
	if !strings.HasPrefix(trimmedPath, "/") {
		trimmedPath = "/" + trimmedPath
	}
	return trimmedPath
}

func (fs *VirtualRootFs) findMountPoint(filepath string) (f afero.Fs, p string) {
	nf, _, np := fs.findMountPoint2(filepath)
	return nf, np
}

func (fs *VirtualRootFs) findMountPoint2(filepath string) (f afero.Fs, mp, p string) {
	name := filepath
	for {
		if v, ok := fs.mountPoints[name]; ok {
			f = v
			mp = name
			p = strings.TrimPrefix(filepath, name)
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			return
		}
		if name == "/" {
			return nil, "", ""
		}
		name = path.Dir(name)
	}
}

func (fs *VirtualRootFs) Mount(mountPath string, vfs *afc.Fsync) {
	fs.mountPoints[mountPath] = vfs
}

func (fs *VirtualRootFs) Unmount(mountPath string) {
	// TODO: need call fs.Unmount
	delete(fs.mountPoints, mountPath)
}

func (fs *VirtualRootFs) Create(name string) (afero.File, error) {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return nil, syscall.EPERM
	}
	return mp.Create(newPath)
}

func (fs *VirtualRootFs) Mkdir(name string, perm os.FileMode) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Mkdir(newPath, perm)
}

func (fs *VirtualRootFs) MkdirAll(name string, perm os.FileMode) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.MkdirAll(newPath, perm)
}

func (fs *VirtualRootFs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *VirtualRootFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		switch name {
		case "/":
			names := []string{afcMountPath, sandboxMountPath}
			return &VFile{absPath: name, names: names}, nil
		case sandboxMountPath + "/":
			name = sandboxMountPath
			fallthrough
		case sandboxMountPath:
			var names []string
			for mountPath, _ := range fs.mountPoints {
				if path.Dir(mountPath) == name {
					names = append(names, path.Base(mountPath))
				}
			}
			return &VFile{absPath: name, names: names}, nil
		default:
			return nil, syscall.EPERM
		}
	}

	return mp.OpenFile(newPath, flag, perm)
}

func (fs *VirtualRootFs) Remove(name string) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Remove(newPath)
}

func (fs *VirtualRootFs) RemoveAll(name string) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.RemoveAll(newPath)
}

func (fs *VirtualRootFs) Rename(oldname, newname string) error {
	mp, point, oldname2 := fs.findMountPoint2(oldname)
	if mp == nil {
		return syscall.EPERM
	}
	newname2 := fs.trimPath(newname, point)
	return mp.Rename(oldname2, newname2)
}

func (fs *VirtualRootFs) Stat(name string) (os.FileInfo, error) {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return afc.NewDirStatInfo(name), nil
	}

	return mp.Stat(newPath)
}

func (fs *VirtualRootFs) Name() string { return "iOSVirtualRootFs" }

func (fs *VirtualRootFs) Chmod(name string, mode os.FileMode) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}
	return mp.Chmod(newPath, mode)
}

func (fs *VirtualRootFs) Chown(name string, uid, gid int) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}
	return mp.Chown(newPath, uid, gid)
}

func (fs *VirtualRootFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Chtimes(newPath, atime, mtime)
}
