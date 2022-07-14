package virtualfs

import (
	"github.com/spf13/afero"
	"os"
)

type VirtualFs interface {
	afero.Fs
	DoMount(mountPath string, vfs VirtualFs)
	MountPoints() map[string]VirtualFs
	ReadDir(absPath string) (fi []os.FileInfo, err error)
}
