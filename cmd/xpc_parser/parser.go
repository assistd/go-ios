package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os/exec"
	"regexp"
)

type ArchTye int

const (
	ArchX86_64 ArchTye = 1
	ArchArm64  ArchTye = 2
)

func parse_arch(output []byte) (string, error) {
	pattern := `architecture (\w+)`
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

func reverseBytes(s []byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// x86_64:
// 000000010000fc8e        69 6c 64 3c 2f 6b 65 79 3e 0a 09 3c 73 74 72 69
// arm64:
// 000000010000fd9e        74636964 2f3c0a3e 73696c70 74 3e 0a
func parseLine(output []byte) ([]byte, error) {
	pattern := `[0-9A-Fa-f]+\s+([^\n]+)`
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return []byte{}, err
	}

	found := reg.FindSubmatch(output)
	if len(output) < 2 {
		return []byte{}, fmt.Errorf("not found uuid, got:%v", found)
	}

	out := make([][]byte, 0)
	slices := bytes.Split(found[1], []byte{' '})
	for _, s := range slices {
		tmp := make([]byte, hex.DecodedLen(len(s)))
		n, _ := hex.Decode(tmp, s)
		if len(s) > 2 {
			reverseBytes(tmp[:n])
		}

		out = append(out, tmp[:n])
	}
	return bytes.Join(out, nil), nil
}

func parse(segname, sectname, binary string) ([][]byte, error) {
	cmd := exec.Command("otool", "-s", segname, sectname, binary)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lists := make([][]byte, 0)
	buf := new(bytes.Buffer)
	bytelines := bytes.Split(bytes.TrimSpace(output), []byte{'\n'})

	flag := 0
	count := 0
	for _, byteline := range bytelines {
		// fmt.Println(string(byteline))
		if bytes.Contains(byteline, []byte(binary)) {
			flag = 1
			lists = append(lists, buf.Bytes())
			buf.Reset()
			if count > 0 {
				fmt.Println("")
			}
			fmt.Println(string(byteline))
			count++
			continue
		}

		switch flag {
		case 1:
			if !bytes.Contains(byteline, []byte("Contents of")) {
				return nil, fmt.Errorf("invalid format: should be 'Contents of ...': line:%v", string(byteline))
			}
			flag = 2
		case 2:
			found, err := parseLine(byteline)
			if err != nil {
				return nil, fmt.Errorf("invalid format: line:%v", byteline)
			}
			buf.Write(found)
			fmt.Print(string(found))
		}
	}

	return lists, nil
}

var segname = flag.String("segname", "__TEXT", "segment name")
var sectname = flag.String("sectname", "__info_plist", "section name: __info_plist | __launchd_plist")
var binary = flag.String("file", "", "binary file path: /Library/PrivilegedHelperTools/*")

func main() {
	flag.Parse()
	if *binary == "" {
		flag.Usage()
		return
	}
	_, err := parse(*segname, *sectname, *binary)
	if err != nil {
		panic(err)
	}
}
