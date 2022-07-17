package afc

import (
	"os"
	"path"
	"time"
)

type StatInfo struct {
	name         string
	stSize       int64
	stBlocks     int64
	stCtime      int64
	stMtime      int64
	stNlink      string
	stIfmt       string
	stLinktarget string
}

func (s *StatInfo) Name() string {
	return s.name
}

func (s *StatInfo) Size() int64 {
	return s.stSize
}

func (s *StatInfo) Mode() os.FileMode {
	if s.stIfmt == "S_IFDIR" {
		return os.ModeDir
	}
	return 0
}

func (s *StatInfo) CTime() time.Time {
	return time.UnixMicro(s.stCtime / 1000)
}

func (s *StatInfo) ModTime() time.Time {
	return time.UnixMicro(s.stMtime / 1000)
}

func (s *StatInfo) Sys() interface{} {
	return s
}

func (s *StatInfo) IsDir() bool {
	return s.stIfmt == "S_IFDIR"
}

func (s *StatInfo) IsLink() bool {
	return s.stIfmt == "S_IFLNK"
}

func NewDirStatInfo(name string) *StatInfo {
	return &StatInfo{
		name:         path.Base(name),
		stSize:       0,
		stBlocks:     0,
		stCtime:      0,
		stMtime:      0,
		stNlink:      "",
		stIfmt:       "S_IFDIR",
		stLinktarget: "",
	}
}
