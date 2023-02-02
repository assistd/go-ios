package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
)

var usbmuxdPath = flag.String("usbmuxd-path", "unix:/var/run/usbmuxd", "usbmuxd path")
var addr = flag.String("addr", "", "remote usbmuxd addr")
var udid = flag.String("udid", "", "remote device udid")
var logPath = flag.String("log_path", "/var/log/", "log directory")

func initLog(udid string) {
	savePath := fmt.Sprintf("%v/wdb/%v", *logPath, udid)
	p := savePath + ".%Y%m%d.log"
	writer, _ := rotatelogs.New(p,
		rotatelogs.WithLinkName(savePath),
		rotatelogs.WithMaxAge(time.Duration(3)*time.Hour*24),
		rotatelogs.WithRotationTime(time.Hour*24),
		rotatelogs.WithLinkName(""),
	)
	formatter := &log.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	}

	log.SetReportCaller(true)
	log.SetFormatter(formatter)
	log.SetLevel(log.InfoLevel)
	log.AddHook(lfshook.NewHook(writer, formatter))
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
	if *addr == "" || *udid == "" {
		log.Panicln("addr param and udid param are required")
	}
	initLog(*udid)
	initUsbmuxd()
	remoteDevice := ioskit.NewRemoteDevice(*addr, *udid)
	log.Infof("connected to remote device: %+v", remoteDevice)
	go func() {
		log.Panicln(remoteDevice.Monitor(context.Background()))
	}()

	muxd := ioskit.NewUsbmuxd(*usbmuxdPath, remoteDevice)

	log.Panicln(muxd.Run())
}
