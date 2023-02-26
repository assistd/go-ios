package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

var udid = flag.String("u", "", "device udid")
var monkey = flag.Bool("m", false, "run monkey test")

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetReportCaller(true)
}

func main() {
	TestDvt()

	flag.Parse()
	if *udid == "" {
		panic("need set udid by -u")
	}
	if *monkey {
		monkeyTest(*udid)
	}
	for {
		err := stablityTest(*udid)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Second * 10)
	}

}
