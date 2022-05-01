package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
	"net/http"
	"path"
	"runtime"

	"github.com/gin-gonic/gin"
)

var usbmuxdPath = flag.String("usbmuxd-path", "/var/run/usbmuxd", "usbmuxd path")
var port = flag.Int("port", 6000, "adbd port base")
var selfPort = flag.Int("httpPort", 8083, "http server port")

func runHttpServer(port uint16, kit *tmuxd) {
	router := gin.Default()

	// This handler will return all adbds' information
	router.GET("/list", func(c *gin.Context) {
		devices := kit.listAll()
		c.String(http.StatusOK, "%s", devices)
	})

	// This handler only return one device information specified by serial
	router.GET("/list/:serial", func(c *gin.Context) {
		serial := c.Param("serial")
		proxy, err := kit.get(serial)
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.String(http.StatusOK, fmt.Sprintf("%d", proxy.Port()))
	})

	_ = router.Run(fmt.Sprintf(":%d", port))
}

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

	ios.SetUsbmuxdSocket("unix", "/var/run/usbmux_real")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kit, err := newTmuxd(*usbmuxdPath, *port)
	if err != nil {
		log.Fatalln("tmuxd create failed: ", err)
	}

	go func() {
		kit.listen()
		err := kit.run(ctx)
		if err != nil {
			log.Fatalln("tmuxd quit")
		}
	}()

	runHttpServer(uint16(*selfPort), kit)
}
