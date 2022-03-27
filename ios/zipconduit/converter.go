package zipconduit

import (
	"bytes"
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	headerMagic = 0x57545354  // WTST
	headerVersion = 0x01
	headerSize = 36
)

type _inner struct {
	Magic          uint32
	Version        uint32
	PayloadLength  uint64
	PayloadMD5     [16]byte
}

type conduitZipHeader struct {
	_inner
	HeaderChecksum uint32
}

func newHeader() *conduitZipHeader {
	return &conduitZipHeader{
		_inner: _inner{
			Magic: headerMagic,
			Version: headerVersion,
		},
	}
}

func (c *conduitZipHeader) fillChecksum() {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, c._inner)
	c.HeaderChecksum = crc32.ChecksumIEEE(buf.Bytes())
}

func (c *conduitZipHeader) isValid() bool {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, c._inner)
	return c.Magic == headerMagic &&
		c.HeaderChecksum == crc32.ChecksumIEEE(buf.Bytes())
}

func (c *conduitZipHeader) bytes() []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, c)
	return buf.Bytes()
}

func IsConduitZip(ipaApp string) (bool, error) {
	f, err := os.Open(ipaApp)
	if err != nil {
		return false, err
	}

	defer f.Close()
	header := &conduitZipHeader{}
	err = binary.Read(f, binary.BigEndian, header)
	if err != nil {
		return false, err
	}

	return header.isValid(), nil
}

func ConvertIpaToConduitZip(ipaApp string, outDir string) (string, error) {
	appName := filepath.Base(ipaApp)
	outName := path.Join(outDir, appName+".conduit")
	outFile, err := os.Create(outName)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	pwd, _ := os.Getwd()
	tmpDir, err := ioutil.TempDir(pwd, "temp")
	if err != nil {
		return "", err
	}

	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.WithFields(log.Fields{"dir": tmpDir}).Warn("failed removing tempdir")
		}
	}()

	_, _, err = Unzip(ipaApp, tmpDir)
	if err != nil {
		return "", err
	}

	header := newHeader()
	header.fillChecksum()
	log.Debug("writing header")
	_, err = outFile.Write(header.bytes())
	if err != nil {
		return "", err
	}

	err = packDirToConduitStream(tmpDir, outFile)
	if err != nil {
		return "", err
	}
	return outName, nil
}

func packDirToConduitStream(dir string, stream io.Writer) error {
	var totalBytes int64
	var unzippedFiles []string
	metainfPath := path.Join(dir, "META-INF")
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if dir != path  && !strings.HasPrefix(path, metainfPath) {
				totalBytes += info.Size()
				unzippedFiles = append(unzippedFiles, path)
			}
			return nil
		})
	if err != nil {
		return err
	}
	metainfFolder, metainfFile, err := addMetaInf(dir, unzippedFiles, uint64(totalBytes))
	if err != nil {
		return err
	}

	log.Debug("writing meta inf")
	err = AddFileToZip(stream, metainfFolder, dir)
	if err != nil {
		return err
	}
	err = AddFileToZip(stream, metainfFile, dir)
	if err != nil {
		return err
	}
	log.Debug("meta inf send successfully")

	for _, file := range unzippedFiles {
		err := AddFileToZip(stream, file, dir)
		if err != nil {
			return err
		}
	}
	log.Debug("files sent, sending central header....")
	_, err = stream.Write(centralDirectoryHeader)
	if err != nil {
		return err
	}
	return nil
}

func PackDirToConduitStream(dir string, stream io.Writer) error {
	header := newHeader()
	header.fillChecksum()
	log.Debug("writing header")
	_, err := stream.Write(header.bytes())
	if err != nil {
		return nil
	}

	return packDirToConduitStream(dir, stream)
}