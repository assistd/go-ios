package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func stablityTest(udid string) error {
	err := info(udid)
	if err != nil {
		return fmt.Errorf("exec [info] command failed:%v", err)
	}
	err = date(udid)
	if err != nil {
		return fmt.Errorf("exec [date] command failed:%v", err)
	}
	err = screenshot(udid)
	if err != nil {
		return fmt.Errorf("exec [screenshot] command failed:%v", err)
	}
	err = installApp(udid)
	if err != nil {
		return fmt.Errorf("exec [install && launch] command failed:%v", err)
	}
	err = xctest(udid)
	if err != nil {
		return fmt.Errorf("exec [xctest] command failed:%v", err)
	}
	err = reboot(udid)
	if err != nil {
		return fmt.Errorf("exec [reboot] command failed:%v", err)
	}
	err = developer(udid)
	if err != nil {
		return fmt.Errorf("exec [developer] command failed:%v", err)
	}
	err = blockCase(udid)
	if err != nil {
		return fmt.Errorf("run block case failed:%v", err)
	}
	syslog(udid)
	return nil
}

func date(udid string) error {
	buf, err := commandWithTimeout(udid, []string{"date"})
	if err != nil {
		return err
	}
	output := strings.Trim(buf.String(), "\n")
	log.Infof("[date] output:%v", output)
	_, err = time.ParseInLocation("2006-01-02 15:04:05", output, time.Local)
	return err
}

func info(udid string) error {
	commands := [][]string{
		{"date"},
		{"info"},
		{"ps"},
		{"applist"},
		{"fsync", "ls", "/"},
	}
	shuffle(commands)
	for _, v := range commands {
		buf, err := commandWithTimeout(udid, v)
		if err != nil {
			return err
		}
		log.Infof("%v output:%v", v, buf.String())
	}
	return nil
}

func screenshot(udid string) error {
	buf, err := commandWithTimeout(udid, []string{"screenshot"})
	if err != nil {
		return err
	}
	log.Infof("[screenshot] output:%v", buf.String())

	clientFile, err := os.OpenFile("screenshot.jpg", os.O_RDONLY, 0755)
	if err != nil {
		return fmt.Errorf("open image failed:[%v]", err)
	}
	defer clientFile.Close()
	buff := make([]byte, 512)
	if _, err = clientFile.Read(buff); err != nil {
		return err
	}
	if http.DetectContentType(buff) == "image/jpeg" {
		return nil
	}
	log.Info("[screenshot] ok")
	return fmt.Errorf("not a image")
}

func installApp(udid string) error {
	buf, err := commandWithTimeout(udid, []string{"install", "demo.ipa"})
	if err != nil {
		return err
	}
	log.Infof("[install] output:%v", buf.String())

	buf1, err := commandWithTimeout(udid, []string{"launch", "com.wetest.demo.db"})
	if err != nil {
		return err
	}
	log.Infof("[launch] output:%v", buf1.String())
	launchOutput := strings.Trim(buf1.String(), "\n")
	launchOutputArr := strings.Split(launchOutput, ":")
	if len(launchOutputArr) == 2 && launchOutputArr[0] == "PID" {
		return nil
	}
	return fmt.Errorf("launch demo app failed")
	log.Infof("[install && launch] ok")
	return nil
}

func xctest(udid string) error {
	c := make(chan error, 1)
	//ctx, cancel := context.WithCancel(context.Background())
	go func() {
		buf, err := command(udid, []string{"xctest", "-B", "com.facebook.WebDriverAgentRunner.xctrunner"})
		if err != nil {
			c <- fmt.Errorf("command [xctest] failed:%v", err)
			return
		}
		log.Infof("[xctest] output:%v", buf.String())
	}()
	select {
	case <-time.After(time.Second * 30):
		go func() {
			_, err := command(udid, []string{"relay", "8100", "8100"})
			if err != nil {
				c <- fmt.Errorf("command [relay] failed:%v", err)
				return
			}
		}()
		time.Sleep(time.Second * 3)
		r, err := http.Get("http://127.0.0.1:8100/status")
		if err != nil {
			return fmt.Errorf("request relay port failed:%v", err)
		}
		if r.StatusCode == http.StatusOK {
			return nil
		}
		return fmt.Errorf("relay failed:%v", err)

	case err := <-c:
		return err
	}

	log.Infof("[xctest && relay] ok")
	return nil
}

func syslog(udid string) {
	parallel := 300
	for i := 0; i < parallel; i++ {
		log.Infof("start %v [syslog] goroutinue", i+1)
		go func() {
			_, err := commandWithTimeout(udid, []string{"syslog"})
			if err != nil {
				panic(err)
			}
		}()
	}
}

func developer(udid string) error {
	err := reboot(udid)
	if err != nil {
		return err
	}
	buf, err := commandWithTimeout(udid, []string{"developer"})
	log.Infof("[developer] output:%v", buf.String())
	return nil
}

func reboot(udid string) error {
	buf, err := commandWithTimeout(udid, []string{"reboot"})
	if err != nil {
		return err
	}
	output := strings.Trim(buf.String(), "\n")
	if output != "Success" {
		return fmt.Errorf("reboot failed")
	}
	log.Infof("[reboot] output:%v", buf.String())
	_, err = commandWithTimeout(udid, []string{"info"})
	if err == nil {
		return fmt.Errorf("reboot failed")
	}
	time.Sleep(time.Minute)
	buf, err = commandWithTimeout(udid, []string{"info"})
	if err != nil {
		return err
	}
	log.Infof("[info] output:%v", buf.String())
	return nil
}

// https://github.com/assistd/go-ios/issues/39
func blockCase(udid string) error {
	for i := 0; i < 5; i++ {
		err := developer(udid)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "idevicediagnostics", "restart")
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		if strings.LastIndex(string(output), "ERROR") >= 0 {
			return fmt.Errorf("idevicediagnostics restart failed:%v", string(output))
		}
		log.Infof("block case run [%v] times,idevicediagnostics output:%v", i+1, string(output))
		err = info(udid)
		if err != nil {
			return err
		}
	}
	log.Infof("block case run ok")
	return nil
}

func commandWithTimeout(udid string, command []string) (*bytes.Buffer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output := bytes.NewBuffer(nil)
	command = append([]string{"-u", udid}, command...)
	cmd := exec.CommandContext(ctx, "tidevice", command...)
	cmd.Stdout = output
	cmd.Stderr = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return output, nil
}

func command(udid string, command []string) (*bytes.Buffer, error) {
	output := bytes.NewBuffer(nil)
	command = append([]string{"-u", udid}, command...)
	cmd := exec.Command("tidevice", command...)
	cmd.Stdout = output
	cmd.Stderr = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return output, nil
}
