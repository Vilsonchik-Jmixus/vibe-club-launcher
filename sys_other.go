//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Non-Windows stubs, used only for development and tests.

func isWindows() bool { return false }

func hideWindow(cmd *exec.Cmd) {}

func freeSpace(dir string) int64 {
	var st syscall.Statfs_t
	for d := dir; ; {
		if err := syscall.Statfs(d, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize)
		}
		p := parentDir(d)
		if p == d {
			return 0
		}
		d = p
	}
}

func parentDir(d string) string {
	for i := len(d) - 1; i > 0; i-- {
		if d[i] == '/' {
			return d[:i]
		}
	}
	return "/"
}

func messageBox(title, text string) { fmt.Fprintln(os.Stderr, title+": "+text) }

func pickFolder(initial string) (string, error) {
	if v := os.Getenv("VC_TEST_PICK"); v != "" {
		return v, nil
	}
	return "", errors.New("выбор папки доступен только в Windows")
}

func openFolder(path string) error  { return nil }
func openURL(u string) error        { fmt.Println("open", u); return nil }
func launchOfficialLauncher() error { return errors.New("только Windows") }
func openAppWindow(url string) (*exec.Cmd, error) {
	fmt.Println("UI:", url)
	return nil, nil
}
