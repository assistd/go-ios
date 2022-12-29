package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"path"
	"runtime"
)

func tcp_to_unix(tcp, unix string) error {
	listener, err := net.Listen("unix", unix)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", unix, err)
	}

	os.Chmod(unix, 0777)
	log.Debugln("listen on: ")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		client, err := net.Dial("tcp", tcp)
		go func() {
			io.Copy(client, conn)
			client.Close()
		}()
		go func() {
			io.Copy(conn, client)
			conn.Close()
		}()
	}
}

func unix_to_tcp(unix, tcp string) error {
	listener, err := net.Listen("tcp", tcp)
	if err != nil {
		return fmt.Errorf("usbmuxd: fail to listen on: %v, error:%v", tcp, err)
	}

	log.Debugln("listen on: ")
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("usbmuxd: fail to listen accept: %v", err)
		}

		client, err := net.Dial("unix", unix)
		go func() {
			io.Copy(client, conn)
		}()
		go func() {
			io.Copy(conn, client)
		}()
	}
}

var mode = flag.String("mode", "unix", "wdbd server port")

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

	switch *mode {
	case "unix":
		panic(tcp_to_unix("127.0.0.1:27015", "/var/run/usbmuxd"))
	case "tcp":
		panic(unix_to_tcp("/var/run/usbmuxd", "127.0.0.1:27015"))
	}
}
