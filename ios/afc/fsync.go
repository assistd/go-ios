package afc

import (
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"io"
	"os"
	"path"
	"path/filepath"
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
	fileInfo, err := fs.Stat(srcPath)
	if err != nil {
		return err
	}

	if fileInfo.IsLink() {
		srcPath = fileInfo.stLinktarget
	}
	fd, err := fs.OpenFile(srcPath, Afc_Mode_RDONLY)
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

	fd, err := fs.OpenFile(dstPath, Afc_Mode_WR)
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
