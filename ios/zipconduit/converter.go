package zipconduit

import (
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

func ConvertIpaToConduitZip(ipaApp string, outDir string) error {
	appName := filepath.Base(ipaApp)
	outName := path.Join(outDir, appName+".conduit")
	outFile, err := os.Create(outName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	pwd, _ := os.Getwd()
	tmpDir, err := ioutil.TempDir(pwd, "temp")
	if err != nil {
		return err
	}

	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.WithFields(log.Fields{"dir": tmpDir}).Warn("failed removing tempdir")
		}
	}()

	_, _, err = Unzip(ipaApp, tmpDir)
	if err != nil {
		return err
	}

	err = packDirToConduitStream(tmpDir, outFile)
	if err != nil {
		return err
	}
	return nil
}

func packDirToConduitStream(dir string, stream io.Writer) error {
	var totalBytes int64
	var unzippedFiles []string
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			totalBytes += info.Size()
			unzippedFiles = append(unzippedFiles, path)
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
