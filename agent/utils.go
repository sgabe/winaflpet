//go:build windows
// +build windows

package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danieljoos/wincred"
	"github.com/mitchellh/go-ps"
	"golang.org/x/sys/windows"
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
		value := strings.Replace(strings.TrimSpace(s[1]), "inf", "0.0", 1)
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

func splitCmdLine(cmdLine string) (string, string) {
	cmdFields := strings.Fields(cmdLine)

	cmd := cmdFields[0]
	args := ""

	if len(cmdFields) > 1 {
		args = strings.Join(cmdFields[1:], " ")
	}

	return cmd, args
}

func joinPath(workingDir string, outputDir string, pathNames ...string) string {
	e := append([]string{outputDir}, pathNames...)

	if !filepath.IsAbs(outputDir) {
		e = append([]string{workingDir}, e...)
	}

	p := filepath.Join(e...)

	return p
}

func readStdout(c chan error, rd *bufio.Reader) {
	for {
		l, _, err := rd.ReadLine()
		if err != nil || err == io.EOF {
			c <- err
		}

		s := string(l)
		if strings.Contains(s, AFL_SUCCESS_MSG) {
			c <- nil
		}

		m := regexp.MustCompile(AFL_FAIL_REGEX).FindStringSubmatch(s)
		if len(m) > 0 {
			c <- errors.New(stripAnsi(m[1]))
		}
	}
}

func sequentialName(name string, fID int) string {
	i := strings.LastIndex(name, ".exe")
	if i == -1 {
		return name
	}

	return fmt.Sprintf("%s%d%s", name[:i], fID, name[i:])
}

func getFuncAddr(path string) string {
	re := regexp.MustCompile(`id_\d{6}_([^_]+)`)
	if m := re.FindStringSubmatch(path); len(m) > 1 {
		return m[1]
	}
	return "Unknown"
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%X", h.Sum(nil)), nil
}

func copyFile(src, dst string) (err error) {
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	if _, err := os.Stat(dst); err == nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	if err = out.Sync(); err != nil {
		out.Close()
		return err
	}

	if err = out.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, dst)
}

func jobCleanup(job windows.Handle, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		windows.CloseHandle(job)
	}()
}
