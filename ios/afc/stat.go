package afc

import (
	"os"
	"time"
)

type statInfo struct {
	name         string
	stSize       int64
	stBlocks     int64
	stCtime      int64
	stMtime      int64
	stNlink      string
	stIfmt       string
	stLinktarget string
}

func (s *statInfo) Name() string {
	return s.name
}

func (s *statInfo) Size() int64 {
	return s.stSize
}

func (s *statInfo) Mode() os.FileMode {
	if s.stIfmt == "S_IFDIR" {
		return os.ModeDir
	}
	return 0
}

func (s *statInfo) CTime() time.Time {
	return time.UnixMicro(s.stCtime / 1000)
}

func (s *statInfo) ModTime() time.Time {
	return time.UnixMicro(s.stMtime / 1000)
}

func (s *statInfo) Sys() interface{} {
	return s
}

func (s *statInfo) IsDir() bool {
	return s.stIfmt == "S_IFDIR"
}

func (s *statInfo) IsLink() bool {
	return s.stIfmt == "S_IFLNK"
}
