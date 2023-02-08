package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
)

var usbmuxdPath = flag.String("usbmuxd-path", "unix:/var/run/usbmuxd", "usbmuxd path")
var addr = flag.String("addr", "", "remote usbmuxd addr")
var udid = flag.String("udid", "", "remote device udid")
var logPath = flag.String("log-path", "/var/log/", "log directory")
var mode = flag.String("mode", "wdbd", "wdb wdbd")
var keepAlive = flag.Bool("keepalive", false, "need keepalive")

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

func main() {
	flag.Parse()
	if *addr == "" || *udid == "" {
		log.Panicln("addr param and udid param are required")
	}
	initLog(*udid)
	remoteDevice := ioskit.NewRemoteDevice(*addr, *udid)
	log.Infof("connected to remote device: %+v", remoteDevice)
	muxd := ioskit.NewUsbmuxd(*usbmuxdPath, *keepAlive, remoteDevice)
	switch *mode {
	case "wdb":
		log.Panicln(muxd.Forward())
	default:
		go func() {
			log.Panicln(remoteDevice.Monitor(context.Background()))
		}()

		log.Panicln(muxd.Run())
	}

}
