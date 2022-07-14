package virtualfs

import (
	"os"
	"syscall"
)

type VirtualFile struct {
	vfs     VirtualFs
	absPath string
	isDir   bool
}

func (f *VirtualFile) Close() (err error) {
	return nil
}

func (f *VirtualFile) Read(p []byte) (n int, err error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *VirtualFile) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *VirtualFile) Seek(offset int64, whence int) (int64, error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *VirtualFile) Write(p []byte) (n int, err error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *VirtualFile) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *VirtualFile) Name() string {
	return f.absPath
}

func (f *VirtualFile) Readdir(count int) (fi []os.FileInfo, err error) {
	if count > 0 {
		return nil, syscall.EPFNOSUPPORT
	}
	return f.vfs.ReadDir(f.absPath)

}

func (f *VirtualFile) Readdirnames(count int) (names []string, err error) {
	if count > 0 {
		return nil, syscall.EPFNOSUPPORT
	}

	fi, err := f.Readdir(count)
	if err != nil {
		return nil, err
	}
	for _, dirs := range fi {
		names = append(names, dirs.Name())
	}
	return names, err
}

func (f *VirtualFile) Stat() (os.FileInfo, error) {
	return f.vfs.Stat(f.absPath)
	//return newDirStat(path.Base(f.absPath)), nil
}

func (f *VirtualFile) Sync() error {
	return nil
}

func (f *VirtualFile) Truncate(size int64) error {
	return syscall.EPFNOSUPPORT
}

func (f *VirtualFile) WriteString(s string) (ret int, err error) {
	return -1, syscall.EPFNOSUPPORT
}
