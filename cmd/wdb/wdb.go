package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
	"path"
	"runtime"
)

var usbmuxdPath = flag.String("usbmuxd-path", "unix:/var/run/usbmuxd", "usbmuxd path")
var port = flag.Int("port", 8083, "wdbd server port")

func initLog() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	log.SetLevel(log.InfoLevel)
}

func main() {
	flag.Parse()
	initLog()

	remoteDevice := ioskit.NewRemoteDevice(
		"192.168.0.47:8083",
		"82d8ccbcd9160681f7fd9d377d8e0dff7c6591a5")
	log.Infof("connected to remote device: %+v", remoteDevice)
	go func() {
		log.Panicln(remoteDevice.Monitor(context.Background()))
	}()

	muxd := ioskit.NewUsbmuxd(*usbmuxdPath, remoteDevice)
	ioskit.SetGlobal(muxd)

	log.Panicln(muxd.Run())
}
