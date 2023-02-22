package main

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"
)

var udid = flag.String("u", "", "device udid")
var monkey = flag.Bool("m", false, "run monkey test")

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		TimestampFormat: time.RFC3339Nano,
		FullTimestamp:   true,
	})
}

func main() {
	TestDeviceInfo()

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
