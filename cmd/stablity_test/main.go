package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

var udid = flag.String("u", "", "device udid")

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		TimestampFormat: time.RFC3339Nano,
		FullTimestamp:   true,
	})
}

func main() {
	flag.Parse()
	if *udid == "" {
		panic("need set udid by -u")
	}
	tideviceCommands := [][]string{
		{"date"},
		{"info"},
		{"set-assistive-touch", "--enabled"},
		{"set-assistive-touch"},
		{"ps"},
		{"screenshot"},
		{"install", "app", "demo.ipa"},
		{"launch", "com.wetest.demo.db"},
		{"xctest", "-B", "com.facebook.WebDriverAgentRunner.xctrunner"}, // 需要预装签名的wda
		{"syslog"},
		{"developer"},
		{"dumpfps"},
		{"fsync", "ls", "/"},
	}
	go rebootDevice(*udid)
	for {
		shuffle(tideviceCommands)
		for _, v := range tideviceCommands {
			runCommand(*udid, v)
			time.Sleep(time.Second * 1)
		}
	}

}

func rebootDevice(udid string) {
	for {
		select {
		case <-time.After(time.Hour * 1):
			runCommand(udid, []string{"reboot"})
		}
	}
}

func runCommand(udid string, command []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log := logrus.WithFields(logrus.Fields{
		"command": command,
	})
	log.Info("---------------start-------------------")
	command = append([]string{"-u", udid}, command...)
	cmd := exec.CommandContext(ctx, "tidevice", command...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	log.Info("---------------finish------------------")

}

func shuffle(slice [][]string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(slice) > 0 {
		n := len(slice)
		randIndex := r.Intn(n)
		slice[n-1], slice[randIndex] = slice[randIndex], slice[n-1]
		slice = slice[:n-1]
	}
}
