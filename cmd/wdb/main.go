package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/danielpaulus/go-ios/wdbd"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
	"path"
	"runtime"
	"time"
)

var mode = flag.String("mode", "wdbd", "wdbd server port")
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

	//ios.SetUsbmuxdSocket("unix", "/var/run/usbmuxd")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch *mode {
	case "wdbd":
		// start wdbd monitor
		wdbdSrv, _ := NewWdbd(*usbmuxdPath)
		go wdbdSrv.Monitor(ctx)

		// start wdbd server
		log.Infof("==========>listen on port %d<==========", *port)
		if err := wdbdSrv.Run(ctx, *port); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	case "wdb":
		ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
		defer cancel2()
		remoteDevice := ioskit.NewRemoteDevice(
			ctx2,
			wdbd.DeviceType_IOS,
			"192.168.0.47:8083",
			"82d8ccbcd9160681f7fd9d377d8e0dff7c6591a5")
		log.Infof("connected to remote device: %+v", remoteDevice)

		go func() {
			log.Panicln(remoteDevice.Monitor(ctx))
		}()
		muxd := ioskit.NewUsbmuxd(*usbmuxdPath)
		muxd.Add(remoteDevice)
		ioskit.SetGlobal(muxd)

		log.Panicln(muxd.Run())
	}
}
