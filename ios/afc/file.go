package afc

import (
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"strings"
	"syscall"
)

type File struct {
	conn    *Connection
	pfd     uint64
	absPath string
	isdir   bool
}

func NewFile(conn *Connection, pfd uint64, absPath string, isdir bool) *File {
	return &File{
		conn:    conn,
		pfd:     pfd,
		absPath: absPath,
		isdir:   isdir,
	}
}
func (f *File) Close() (err error) {
	if !f.isdir {
		return f.conn.CloseFile(f.pfd)
	}
	return nil
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.conn.ReadFile(f.pfd, p)
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	return -1, syscall.EPFNOSUPPORT //syscall.EAFNOSUPPORT
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	return f.conn.SeekFile(f.pfd, offset, whence)
}

func (f *File) Write(p []byte) (n int, err error) {
	return f.conn.WriteFile(f.pfd, p)
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, syscall.EPFNOSUPPORT
}

func (f *File) Name() string {
	return f.absPath
}

// Readdir must return nil
func (f *File) Readdir(count int) (fi []os.FileInfo, err error) {
	if count > 0 {
		log.Fatalln("not support count > 0")
	}

	files, _ := f.conn.ReadDir(f.absPath)
	for _, entry := range files {
		fileInfo, err := f.conn.Stat(path.Join(f.absPath, entry))
		if err != nil {
			if strings.Contains(err.Error(), getError(Afc_Err_PermDenied).Error()) {
				log.Errorf("Readdir: %v", err)
				fileInfo = &StatInfo{
					name:         entry,
					stSize:       0,
					stBlocks:     0,
					stCtime:      0,
					stMtime:      0,
					stNlink:      "",
					stIfmt:       "",
					stLinktarget: "",
				}
			}
		}
		fi = append(fi, fileInfo)
	}
	return
}

// Readdirnames must return nil
func (f *File) Readdirnames(count int) (names []string, err error) {
	if count > 0 {
		log.Fatalln("not support count > 0")
	}
	files, err := f.conn.ReadDir(f.absPath)
	return files, nil
}

func (f *File) Stat() (os.FileInfo, error) {
	// FIXME: may be out of date
	return f.conn.Stat(f.absPath)
}

func (f *File) Sync() error {
	return nil
}

func (f *File) Truncate(size int64) error {
	return f.conn.TruncateFile(f.pfd, size)
}

func (f *File) WriteString(s string) (ret int, err error) {
	return -1, syscall.EPFNOSUPPORT
}
