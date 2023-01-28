package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/danielpaulus/go-ios/ios/debugproxy"
	"github.com/danielpaulus/go-ios/wdbd/ioskit"
	log "github.com/sirupsen/logrus"
)

func initLog() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	log.SetLevel(log.InfoLevel)
}

func proxy() error {
	id := 0

	listener, err := net.Listen("tcp", "127.0.39.237:62078")
	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("error with connection: %e", err)
		}
		os.MkdirAll("dumps", os.ModePerm)
		connectionPath := filepath.Join("./dumps", "conn-"+strconv.Itoa(id)+"-"+time.Now().Format("2006.01.02-15.04.05.000"))
		id++

		os.MkdirAll(connectionPath, os.ModePerm)

		binfile := filepath.Join(connectionPath, "protocol.bin")
		f, err := os.OpenFile(binfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		textfile := filepath.Join(connectionPath, "protocol.txt")
		conn = debugproxy.NewDumpingConn(textfile, conn)
		if err != nil {
			conn.Close()
			f.Close()
			panic(err)
		}

		go func() {
			n, _ := io.Copy(f, conn)
			if n == 0 {
				os.Remove(binfile)
			}
			f.Close()
			conn.Close()
		}()
	}
}

func main() {
	flag.Parse()
	initLog()

	// fe32ecec58d608c8735f7f8ca67ca99bdea10ee3
	// 8a8358e12e0306cc804f4367d9152fb795e3b561
	// 00008110-00142C9A3642801E
	// iPhone13pro 00008110-000515511491801E
	remoteDevice := ioskit.NewRemoteDevice(
		"192.168.0.192:27016",
		"00008110-00142C9A3642801E",
	)
	log.Infof("connected to remote device: %+v", remoteDevice)

	ip := "127.0.39.237"
	remoteDevice.Listen(ioskit.NewDeviceListener(
		func(ctx context.Context, d ioskit.DeviceEntry) {
			go func() {
				provider, err := ioskit.NewProvider(ip, remoteDevice)
				if err != nil {
					panic(err)
				}
				panic(provider.Run())
			}()

			// start mdns proxy
			go func() {
				//TODO: Wait for provider running
				time.Sleep(time.Second * 1)
				values, err := remoteDevice.GetInfo()
				if err != nil {
					panic(err)
				}

				proxy := NewMdnsProxy(values.WiFiAddress, values.UniqueDeviceID, ip)
				proxy.Register()
			}()
		},

		func(ctx context.Context, d ioskit.DeviceEntry) {
			// TODO: should cancal all goroutines and close all resources
			panic("device is removed")
		},
	))

	log.Panicln(remoteDevice.Monitor(context.Background()))
}
