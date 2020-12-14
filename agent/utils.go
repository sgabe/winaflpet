// +build windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/danieljoos/wincred"
	"github.com/mitchellh/go-ps"
)

const (
	WINCRED_NAME = "WinAFL_Pet_Agent"
)

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func stripAnsi(s string) string {
	ansi := "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(s, "")
}

func killProcess(p ps.Process) error {
	proc, err := os.FindProcess(p.Pid())
	if err != nil {
		logger.Error(err)
		return err
	}

	err = proc.Kill()
	if err != nil {
		logger.Error(err)
		return err
	}

	logger.Infof("Killed %s process (PID %d, PPID %d)\n", p.Executable(), p.Pid(), p.PPid())

	return nil
}

func parseStats(content string) (Stats, error) {
	var stats Stats
	var fields = make(map[string]interface{})

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			break
		}

		s := strings.Split(line, ":")
		name := strings.TrimSpace(s[0])
		value := strings.TrimSpace(s[1])
		fields[name] = value

		if strings.Contains(value, ".") {
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				fields[name] = f
			}
		} else if i, err := strconv.Atoi(value); err == nil {
			fields[name] = i
		}
	}

	b, err := json.Marshal(fields)
	if err != nil {
		logger.Error(err)
		return stats, err
	}

	if err := json.Unmarshal([]byte(b), &stats); err != nil {
		logger.Error(err)
		return stats, err
	}

	return stats, nil
}

func genKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	k := hex.EncodeToString(b)
	fmt.Println("\nSecret key of service account:", k)
	return k
}

func initKey() error {
	cred := wincred.NewGenericCredential(WINCRED_NAME)
	cred.CredentialBlob = []byte(genKey())
	return cred.Write()
}

func getKey() (string, error) {
	cred, err := wincred.GetGenericCredential(WINCRED_NAME)
	if err != nil {
		return "", err
	}

	return string(cred.CredentialBlob), nil
}

func delKey() error {
	cred, err := wincred.GetGenericCredential(WINCRED_NAME)
	if err != nil {
		return err
	}
	return cred.Delete()
}
