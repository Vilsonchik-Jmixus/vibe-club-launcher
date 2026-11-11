//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const createNoWindow = 0x08000000

func isWindows() bool { return true }

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpEx = kernel32.NewProc("GetDiskFreeSpaceExW")
	user32              = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW     = user32.NewProc("MessageBoxW")
)

func freeSpace(dir string) int64 {
	d := dir
	for {
		if _, err := os.Stat(d); err == nil {
			break
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	ptr, err := syscall.UTF16PtrFromString(d)
	if err != nil {
		return 0
	}
	var free, total, totalFree uint64
	r, _, _ := procGetDiskFreeSpEx.Call(uintptr(unsafe.Pointer(ptr)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
	if r == 0 {
		return 0
	}
	return int64(free)
}

func messageBox(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x40)
}

const psPrelude = `[Console]::OutputEncoding=[Text.Encoding]::UTF8;Add-Type -AssemblyName System.Windows.Forms;$o=New-Object System.Windows.Forms.Form -Property @{TopMost=$true;ShowInTaskbar=$false;Width=0;Height=0;StartPosition='CenterScreen'};$o.Show();$o.Hide();`

func pickFolder(initial string) (string, error) {
	script := psPrelude + `$d=New-Object System.Windows.Forms.FolderBrowserDialog;$d.Description='Выбери папку для сборки Vibe Club';$d.ShowNewFolderButton=$true;$d.SelectedPath=$env:VC_INITIAL;if($d.ShowDialog($o) -eq 'OK'){[Console]::Out.Write($d.SelectedPath)}`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "VC_INITIAL="+initial)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func openFolder(path string) error {
	_ = os.MkdirAll(path, 0o755)
	return exec.Command("explorer.exe", path).Start()
}

func openURL(u string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", u).Start()
}

// launchOfficialLauncher starts the Minecraft Launcher (classic install, Xbox app or Store).
// errNoOfficialLauncher is shown when the official Minecraft Launcher is not installed.
var errNoOfficialLauncher = errors.New("NO_OFFICIAL_LAUNCHER")

func launchOfficialLauncher() error {
	var cands []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if v := os.Getenv(env); v != "" {
			cands = append(cands, filepath.Join(v, "Minecraft Launcher", "MinecraftLauncher.exe"))
		}
	}
	if sd := os.Getenv("SystemDrive"); sd != "" {
		cands = append(cands, filepath.Join(sd+`\`, "XboxGames", "Minecraft Launcher", "Content", "Minecraft.exe"))
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			appendLog("Запускаю " + c)
			return exec.Command(c).Start()
		}
	}
	// Microsoft Store / Xbox app version: only if the package is really installed, otherwise
	// Explorer shows a confusing "Windows cannot find" dialog.
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		if _, err := os.Stat(filepath.Join(la, "Packages", "Microsoft.4297127D64EC6_8wekyb3d8bbwe")); err == nil {
			appendLog("Запускаю лаунчер Minecraft из Microsoft Store")
			return exec.Command("explorer.exe", `shell:AppsFolder\Microsoft.4297127D64EC6_8wekyb3d8bbwe!Minecraft`).Start()
		}
	}
	appendLog("Официальный лаунчер Minecraft не найден")
	return errNoOfficialLauncher
}

func findEdge() string {
	var c []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		if v := os.Getenv(env); v != "" {
			c = append(c, filepath.Join(v, "Microsoft", "Edge", "Application", "msedge.exe"))
		}
	}
	for _, p := range c {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// openAppWindow shows the UI in a chromeless Edge window with its own profile, so closing the
// window ends that process and the launcher can exit with it.
func openAppWindow(url string) (*exec.Cmd, error) {
	edge := findEdge()
	if edge == "" {
		return nil, openURL(url)
	}
	prof := filepath.Join(appDataDir(), "ui-profile")
	cmd := exec.Command(edge,
		"--app="+url,
		"--user-data-dir="+prof,
		"--window-size=1296,759",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate,msEdgeSidebarV2,msUndersideButton",
		"--disable-sync",
	)
	if err := cmd.Start(); err != nil {
		return nil, openURL(url)
	}
	return cmd, nil
}

var errNoPicker = errors.New("NO_PICKER")

func installDesktopEntry() {}
