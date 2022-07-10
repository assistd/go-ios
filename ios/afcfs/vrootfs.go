package afcfs

import (
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/crashreport"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"os"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	afcMountPath          = "/afc"
	crashreportsMountPath = "/crashreports"
	sandboxMountPath      = "/apps"
	documentsPath         = "/Documents"
	documentsDirName      = "Documents"
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

	crashReportFs, err := crashreport.NewFsync(device)
	if err != nil {
		return nil, err
	}
	rootFs.Mount(crashreportsMountPath, crashReportFs)
	return rootFs, nil
}

func (fs *VirtualRootFs) umountAppsSandbox() {
	for name, _ := range fs.mountPoints {
		if strings.HasPrefix(name, sandboxMountPath) {
			delete(fs.mountPoints, name)
		}
	}
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
		//
		// Firstly, we get Container permission.
		// If Container permission denied, we get Documents permission.
		// Generally, we can get both Container and Documents permissions from debugging apps,
		// while only Documents permission can be got from released apps(Enterprise apps, AppStore apps...)
		// sign type:
		//  - Apple iPhone OS Application Signing
		//  - iPhone Distribution: Tencent Technology (Shenzhen) Co., Ltd
		if app.SignerIdentity == "Apple iPhone OS Application Signing" ||
			strings.HasPrefix(app.SignerIdentity, "iPhone Distribution:") {
			sandboxFs, err := afc.NewHouseArrestDocumentFs(fs.device, app.CFBundleIdentifier)
			if err == nil {
				log.Infoln("mount", app.CFBundleIdentifier)
				fs.Mount(path.Join(sandboxMountPath, app.CFBundleIdentifier, documentsDirName), sandboxFs)
			} else {
				log.Errorln("mount", app.CFBundleIdentifier, err)
			}
			continue
		}
		// other SignerIdentity
		var sandboxFs *afc.Fsync
		sandboxFs, err = afc.NewHouseArrestContainerFs(fs.device, app.CFBundleIdentifier)
		if err == nil {
			log.Infoln("mount", app.CFBundleIdentifier, "container")
			fs.Mount(path.Join(sandboxMountPath, app.CFBundleIdentifier), sandboxFs)
			continue
		}
		log.Warnf("mount %v container error:%v. mounting documents...", app.CFBundleIdentifier, err)
		sandboxFs, err = afc.NewHouseArrestDocumentFs(fs.device, app.CFBundleIdentifier)
		if err == nil {
			log.Infoln("mount", app.CFBundleIdentifier, "document")
			fs.Mount(path.Join(sandboxMountPath, app.CFBundleIdentifier, documentsDirName), sandboxFs)
			continue
		}
		log.Errorf("mount %v documents error:%v", app.CFBundleIdentifier, err)
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

func (fs *VirtualRootFs) findMountPoint2(filepath string) (f afero.Fs, mountPoint, newPath string) {
	for mp, f := range fs.mountPoints {
		if strings.HasPrefix(filepath, mp) {
			// When mount as VendDocuments, TrimPrefix trims away the the mount point "/app/<bundle_id>/Documents"
			// from the filepath and only a "/" is left. We have to prepend a /Documents in the path.
			if f.FsType == afc.HouseArrestDocumentFs {
				np := strings.TrimPrefix(filepath, path.Join(sandboxMountPath, f.BundleId))
				return f, mp, np
			}
			np := strings.TrimPrefix(filepath, mp)
			return f, mp, np
		}
	}
	return nil, "", ""
}

func (fs *VirtualRootFs) Mount(mountPath string, vfs *afc.Fsync) {
	fs.mountPoints[mountPath] = vfs
}

func (fs *VirtualRootFs) Unmount(mountPath string) {
	// TODO: need call fs.Unmount
	delete(fs.mountPoints, mountPath)
}

func winPathToUnix(name string) string {
	if runtime.GOOS == "windows" {
		name = strings.ReplaceAll(name, "\\", "/")
	}
	return name
}

func (fs *VirtualRootFs) Create(name string) (afero.File, error) {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return nil, syscall.EPERM
	}
	return mp.Create(newPath)
}

func (fs *VirtualRootFs) Mkdir(name string, perm os.FileMode) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Mkdir(newPath, perm)
}

func (fs *VirtualRootFs) MkdirAll(name string, perm os.FileMode) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.MkdirAll(newPath, perm)
}

func (fs *VirtualRootFs) Open(name string) (afero.File, error) {
	name = winPathToUnix(name)

	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *VirtualRootFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name = winPathToUnix(name)

	f, newPath := fs.findMountPoint(name)
	if f != nil {
		return f.OpenFile(newPath, flag, perm)
	}

	log.Infoln("OpenFile", name)
	// open the file under '/'
	if name == "/" {
		names := []string{crashreportsMountPath, sandboxMountPath, afcMountPath}
		return &VFile{absPath: name, names: names}, nil
	} else if name == sandboxMountPath || name == sandboxMountPath+"/" {
		fs.umountAppsSandbox()
		_ = fs.mountAppsSandbox()
		var names []string
		for _, s := range fs.mountPoints {
			if s.FsType == afc.HouseArrestDocumentFs ||
				s.FsType == afc.HouseArrestContainerFs {
				// We extract the bundleIDs from the mounted paths to be
				// the VFile -- the /apps dir -- child dir names.
				names = append(names, s.BundleId)
			}
		}
		return &VFile{absPath: name, names: names}, nil
	} else {
		// check if open the /app/<bundle_id>/
		for _, s := range fs.mountPoints {
			if s.FsType == afc.HouseArrestDocumentFs {
				parentDir := path.Join(sandboxMountPath, s.BundleId)
				if name == parentDir+"/" || name == parentDir {
					return &VFile{absPath: name, names: []string{documentsDirName}}, nil
				}
			}
		}
	}

	return nil, syscall.EPERM
}

func (fs *VirtualRootFs) Remove(name string) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Remove(newPath)
}

func (fs *VirtualRootFs) RemoveAll(name string) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.RemoveAll(newPath)
}

func (fs *VirtualRootFs) Rename(oldname, newname string) error {
	oldname = winPathToUnix(oldname)
	newname = winPathToUnix(newname)

	mp, point, oldname2 := fs.findMountPoint2(oldname)
	if mp == nil {
		return syscall.EPERM
	}
	newname2 := fs.trimPath(newname, point)
	return mp.Rename(oldname2, newname2)
}

func (fs *VirtualRootFs) Stat(name string) (os.FileInfo, error) {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return afc.NewDirStatInfo(name), nil
	}

	return mp.Stat(newPath)
}

func (fs *VirtualRootFs) Name() string { return "iOSVirtualRootFs" }

func (fs *VirtualRootFs) Chmod(name string, mode os.FileMode) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}
	return mp.Chmod(newPath, mode)
}

func (fs *VirtualRootFs) Chown(name string, uid, gid int) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}
	return mp.Chown(newPath, uid, gid)
}

func (fs *VirtualRootFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = winPathToUnix(name)

	mp, newPath := fs.findMountPoint(name)
	if mp == nil {
		return syscall.EPERM
	}

	return mp.Chtimes(newPath, atime, mtime)
}
