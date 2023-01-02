package main

import (
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
	log "github.com/sirupsen/logrus"
)

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

	/*
		remoteDevice := ioskit.NewRemoteDevice(
			"192.168.0.192:27016",
			"8a8358e12e0306cc804f4367d9152fb795e3b561")
		log.Infof("connected to remote device: %+v", remoteDevice)
		go func() {
			log.Panicln(remoteDevice.Monitor(context.Background()))
		}()
	*/

	panic(proxy())
}
