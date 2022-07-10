package afc

import (
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	_ "github.com/spf13/afero"
	"io"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type Fsync struct {
	*Connection
}

func New(device ios.DeviceEntry) (*Fsync, error) {
	conn, err := NewConn(device)
	if err != nil {
		return nil, err
	}
	return &Fsync{conn}, nil
}

func NewFsyncFromConn(devConn ios.DeviceConnectionInterface) *Fsync {
	return &Fsync{&Connection{deviceConn: devConn}}
}

//ListFiles returns all files in the given directory, matching the pattern.
//Example: ListFiles(".", "*") returns all files and dirs in the current path the afc connection is in
func (fs *Fsync) ListFiles(cwd string, matchPattern string) ([]string, error) {
	files, err := fs.ReadDir(cwd)
	if err != nil {
		return nil, err
	}

	var filteredFiles []string
	for _, f := range files {
		if f == "" {
			continue
		}
		matches, err := filepath.Match(matchPattern, f)
		if err != nil {
			log.Warn("error while matching pattern", err)
		}
		if matches {
			filteredFiles = append(filteredFiles, f)
		}
	}
	return filteredFiles, nil
}

func (fs *Fsync) TreeView(dpath string, prefix string, treePoint bool) error {
	fileInfo, err := fs.Stat(dpath)
	if err != nil {
		return err
	}

	namePrefix := "`--"
	if !treePoint {
		namePrefix = "|--"
	}
	tPrefix := prefix + namePrefix
	if !fileInfo.IsDir() {
		//return fmt.Errorf("error: %v is not dir", dpath)
		fmt.Printf("%s %s\n", tPrefix, filepath.Base(dpath))
		return nil
	}

	fmt.Printf("%s %s/\n", tPrefix, filepath.Base(dpath))
	fileList, err := fs.ReadDir(dpath)
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
		err = fs.TreeView(nPath, rp, tp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (fs *Fsync) PullFile(srcPath, dstPath string) error {
	fileInfo, err := fs.Connection.Stat(srcPath)
	if err != nil {
		return err
	}

	if fileInfo.IsLink() {
		srcPath = fileInfo.stLinktarget
	}
	fd, err := fs.Connection.OpenFile(srcPath, Afc_Mode_RDONLY)
	if err != nil {
		return err
	}
	defer fs.CloseFile(fd)

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	leftSize := fileInfo.stSize
	maxReadSize := 64 * 1024
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, fd)
	binary.LittleEndian.PutUint64(data[8:], uint64(maxReadSize))
	for leftSize > 0 {
		response, err := fs.request(Afc_operation_file_read, data, nil)
		if err != nil {
			return err
		}
		leftSize -= int64(len(response.Payload))
		f.Write(response.Payload)
	}
	return nil
}

func (fs *Fsync) Pull(srcPath, dstPath string) error {
	fileInfo, err := fs.Stat(srcPath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return fs.PullFile(srcPath, dstPath)
	}
	ret, _ := ios.PathExists(dstPath)
	if !ret {
		err = os.MkdirAll(dstPath, 0755)
		if err != nil {
			return err
		}
	}
	fileList, err := fs.ReadDir(srcPath)
	if err != nil {
		return err
	}
	for _, v := range fileList {
		sp := path.Join(srcPath, v)
		dp := path.Join(dstPath, v)
		err = fs.Pull(sp, dp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (fs *Fsync) Push(srcPath, dstPath string) error {
	ret, _ := ios.PathExists(srcPath)
	if !ret {
		return fmt.Errorf("%s: no such file", srcPath)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if fileInfo, _ := fs.Stat(dstPath); fileInfo != nil {
		if fileInfo.IsDir() {
			dstPath = path.Join(dstPath, filepath.Base(srcPath))
		}
	}

	fd, err := fs.Connection.OpenFile(dstPath, Afc_Mode_WR)
	if err != nil {
		return err
	}
	defer fs.CloseFile(fd)

	maxWriteSize := 64 * 1024
	chunk := make([]byte, maxWriteSize)
	for {
		n, err := f.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, fd)
		_, err = fs.request(Afc_operation_file_write, data, chunk[0:n])
		if err != nil {
			return err
		}
	}
	return nil
}

func (fs *Fsync) Name() string { return "iosfs" }

func (fs *Fsync) Create(name string) (afero.File, error) {
	fd, err := fs.Connection.OpenFile(name, Afc_Mode_WR) // O_RDWR | O_CREAT | O_TRUNC
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	return &File{pfd: fd, absPath: name, conn: fs.Connection}, nil
}

func (fs *Fsync) Mkdir(name string, perm os.FileMode) error {
	return fs.MakeDir(name)
}

func (fs *Fsync) MkdirAll(path string, perm os.FileMode) error {
	return syscall.EPERM
}

func (fs *Fsync) Open(name string) (afero.File, error) {
	fd, err := fs.Connection.OpenFile(name, Afc_Mode_RDONLY)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}

	return &File{pfd: fd, absPath: name, conn: fs.Connection}, nil
}

// OpenFile see https://github.com/libimobiledevice/ifuse/blob/master/src/ifuse.c#L177
func (fs *Fsync) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	var afcFlags uint64
	if flag&os.O_RDONLY != 0 {
		afcFlags = Afc_Mode_RDONLY
	} else if flag&os.O_WRONLY != 0 {
		if flag&os.O_TRUNC != 0 {
			afcFlags = Afc_Mode_WRONLY
		} else if flag&os.O_APPEND != 0 {
			afcFlags = Afc_Mode_APPEND
		} else {
			afcFlags = Afc_Mode_RW
		}
	} else if flag&os.O_RDWR == os.O_RDWR {
		if flag&os.O_TRUNC != 0 {
			afcFlags = Afc_Mode_WR
		} else if flag&os.O_APPEND != 0 {
			afcFlags = Afc_Mode_RDAPPEND
		} else {
			afcFlags = Afc_Mode_RW
		}
	}

	fd, err := fs.Connection.OpenFile(name, afcFlags)
	if err != nil {
		return nil, err
	}

	return &File{pfd: fd, absPath: name, conn: fs.Connection}, nil
}

func (fs *Fsync) Remove(name string) error {
	return fs.RemovePath(name)
}

func (fs *Fsync) RemoveAll(path string) error {
	return fs.RemovePathAndContents(path)
}

func (fs *Fsync) Rename(oldname, newname string) error {
	return fs.RenamePath(oldname, newname)
}

func (fs *Fsync) Stat(name string) (os.FileInfo, error) {
	return fs.Connection.Stat(name)
}

func (fs *Fsync) Chmod(name string, mode os.FileMode) error {
	return syscall.EPERM
}

func (fs *Fsync) Chown(name string, uid, gid int) error {
	return syscall.EPERM
}

func (fs *Fsync) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return syscall.EPERM
}
