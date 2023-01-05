package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
)

var usbmuxdPath = flag.String("usbmuxd-path", "unix:/var/run/usbmuxd", "usbmuxd path")
var addr = flag.String("addr", "", "remote usbmuxd addr")
var udid = flag.String("udid", "", "remote device udid")

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

func initUsbmuxd() {
	// 处理已存在的usbmuxd socket
	if runtime.GOOS != "windows" {
		if fileInfo, _ := os.Stat(ios.DefaultUsbmuxdSocket); fileInfo != nil {
			bak := fmt.Sprintf("%v.bak", ios.DefaultUsbmuxdSocket)
			_ = os.Rename(ios.DefaultUsbmuxdSocket, bak)
		}
	}
}

func main() {
	flag.Parse()
	initLog()
	initUsbmuxd()
	if *addr == "" || *udid == "" {
		log.Panicln("addr param and udid param are required")
	}
	remoteDevice := ioskit.NewRemoteDevice(*addr, *udid)
	log.Infof("connected to remote device: %+v", remoteDevice)
	go func() {
		log.Panicln(remoteDevice.Monitor(context.Background()))
	}()

	muxd := ioskit.NewUsbmuxd(*usbmuxdPath, remoteDevice)
	ioskit.SetGlobal(muxd)

	log.Panicln(muxd.Run())
}
