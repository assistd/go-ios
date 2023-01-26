package mackit

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func GetUdid() (string, error) {
	cmdStr := "ioreg -d2 -c IOPlatformExpertDevice"
	cmdArray := strings.Split(cmdStr, " ")
	cmd := exec.Command(cmdArray[0], cmdArray[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	pattern := `"IOPlatformUUID" = "([^"\n]*)"`
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	found := reg.FindSubmatch(output)
	if len(output) < 2 {
		return "", fmt.Errorf("not found uuid, got:%v", found)
	}

	return string(found[1]), nil
}
