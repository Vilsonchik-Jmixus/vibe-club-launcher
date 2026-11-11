package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func minecraftDir() string {
	if v := os.Getenv("APPDATA"); v != "" {
		return filepath.Join(v, ".minecraft")
	}
	home, _ := os.UserHomeDir()
	std := filepath.Join(home, ".minecraft")
	if _, err := os.Stat(std); err != nil {
		// Flatpak launcher keeps its data in its sandbox.
		fp := filepath.Join(home, ".var", "app", "com.mojang.Minecraft", ".minecraft")
		if _, err := os.Stat(fp); err == nil {
			return fp
		}
	}
	return std
}

func loaderVersionID(m *Manifest) string {
	return m.Loader.Type + "-" + m.Loader.Version // neoforge-21.1.265
}

func loaderInstalled(m *Manifest) bool {
	id := loaderVersionID(m)
	_, err := os.Stat(filepath.Join(minecraftDir(), "versions", id, id+".json"))
	return err == nil
}

func ensureLoaderAndProfile(j *Job, m *Manifest) error {
	if m.Loader.Type != "neoforge" || m.Loader.Version == "" {
		return errors.New("в списке сборки не указан NeoForge")
	}
	mc := minecraftDir()
	if err := os.MkdirAll(mc, 0o755); err != nil {
		return err
	}
	// The NeoForge installer refuses to run without a launcher_profiles.json.
	lp := filepath.Join(mc, "launcher_profiles.json")
	if _, err := os.Stat(lp); err != nil {
		_ = writeFileAtomic(lp, []byte(`{"profiles":{},"settings":{},"version":3}`))
	}
	if !loaderInstalled(m) {
		java, err := findOrInstallJava(j)
		if err != nil {
			return err
		}
		u := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", m.Loader.Version, m.Loader.Version)
		inst := filepath.Join(appDataDir(), "cache", fmt.Sprintf("neoforge-%s-installer.jar", m.Loader.Version))
		if _, err := os.Stat(inst); err != nil {
			j.logf("Скачиваю установщик NeoForge %s", m.Loader.Version)
			j.set(func(s *JobStatus) { s.PhaseText = "Скачиваю NeoForge…" })
			if err := simpleDownload(j, u, inst); err != nil {
				return fmt.Errorf("не удалось скачать NeoForge: %w", err)
			}
		}
		j.set(func(s *JobStatus) { s.PhaseText = "Устанавливаю NeoForge (1–3 минуты)…" })
		j.logf("Запускаю установщик NeoForge: %s", java)
		cmd := exec.Command(java, "-jar", inst, "--install-client", mc)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(inst) // the installer writes installer.log into the working dir
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err = cmd.Run()
		if !loaderInstalled(m) {
			// older installers use --installClient
			out.Reset()
			cmd2 := exec.Command(java, "-jar", inst, "--installClient", mc)
			hideWindow(cmd2)
			cmd2.Dir = filepath.Dir(inst)
			cmd2.Stdout, cmd2.Stderr = &out, &out
			err = cmd2.Run()
		}
		tail := out.String()
		if len(tail) > 1500 {
			tail = tail[len(tail)-1500:]
		}
		_ = os.WriteFile(filepath.Join(appDataDir(), "neoforge-install.log"), out.Bytes(), 0o644)
		if !loaderInstalled(m) {
			j.logf("Вывод установщика: %s", tail)
			if err == nil {
				err = errors.New("установщик завершился без результата")
			}
			return fmt.Errorf("NeoForge не установился: %v (подробности в neoforge-install.log)", err)
		}
		j.logf("NeoForge %s установлен", m.Loader.Version)
	}
	return writeProfile(m)
}

const profileKey = "vibeclub"

func profileJavaArgs(m *Manifest) string {
	gb := getSettings().MemoryGB
	extra := m.JavaArgs
	if extra == "" {
		extra = "-XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=200 -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=40 -XX:G1HeapRegionSize=8M -XX:G1ReservePercent=20"
	}
	return fmt.Sprintf("-Xmx%dG -Xms2G %s", gb, extra)
}

// writeProfile adds/updates the "Vibe Club" profile in the official launcher (both the classic
// launcher_profiles.json and the Microsoft Store variant if present).
func writeProfile(m *Manifest) error {
	mc := minecraftDir()
	files := []string{filepath.Join(mc, "launcher_profiles.json")}
	if _, err := os.Stat(filepath.Join(mc, "launcher_profiles_microsoft_store.json")); err == nil {
		files = append(files, filepath.Join(mc, "launcher_profiles_microsoft_store.json"))
	}
	icon := "Furnace"
	if b, err := webFS.ReadFile("web/assets/profile-icon.png"); err == nil {
		icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for _, p := range files {
		root := map[string]any{}
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &root)
		}
		profiles, _ := root["profiles"].(map[string]any)
		if profiles == nil {
			profiles = map[string]any{}
		}
		prof, _ := profiles[profileKey].(map[string]any)
		if prof == nil {
			prof = map[string]any{"created": now}
		}
		prof["name"] = "Vibe Club"
		prof["type"] = "custom"
		prof["icon"] = icon
		prof["lastVersionId"] = loaderVersionID(m)
		prof["gameDir"] = getSettings().InstallDir
		prof["javaArgs"] = profileJavaArgs(m)
		prof["lastUsed"] = now
		profiles[profileKey] = prof
		root["profiles"] = profiles
		if _, ok := root["version"]; !ok {
			root["version"] = 3
		}
		b, _ := json.MarshalIndent(root, "", "  ")
		if err := writeFileAtomic(p, b); err != nil {
			return fmt.Errorf("не удалось записать профиль лаунчера: %w", err)
		}
	}
	return nil
}

func profileExists() bool {
	b, err := os.ReadFile(filepath.Join(minecraftDir(), "launcher_profiles.json"))
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte(`"`+profileKey+`"`))
}

// ---------------------------------------------------------------- Java

var javaVerRe = regexp.MustCompile(`version "(\d+)`)

func javaMajor(java string) int {
	cmd := exec.Command(java, "-version")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	mm := javaVerRe.FindSubmatch(out)
	if mm == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(mm[1]))
	return n
}

func javaCandidates() []string {
	var c []string
	exe := "java.exe"
	if !isWindows() {
		exe = "java"
	}
	if ms, _ := filepath.Glob(filepath.Join(appDataDir(), "java", "*", "bin", exe)); len(ms) > 0 {
		c = append(c, ms...)
	}
	plat := "windows-x64"
	if !isWindows() {
		plat = "linux"
	}
	rt := filepath.Join("runtime", "java-runtime-delta", plat, "java-runtime-delta", "bin", exe)
	c = append(c, filepath.Join(minecraftDir(), rt))
	if !isWindows() {
		home, _ := os.UserHomeDir()
		c = append(c, filepath.Join(home, ".var", "app", "com.mojang.Minecraft", ".minecraft", rt))
		if ms, _ := filepath.Glob("/usr/lib/jvm/*21*/bin/java"); len(ms) > 0 {
			c = append(c, ms...)
		}
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		c = append(c, filepath.Join(la, "Packages", "Microsoft.4297127D64EC6_8wekyb3d8bbwe", "LocalCache", "Local", rt))
	}
	if jh := os.Getenv("JAVA_HOME"); jh != "" {
		c = append(c, filepath.Join(jh, "bin", exe))
	}
	if p, err := exec.LookPath("java"); err == nil {
		c = append(c, p)
	}
	return c
}

func findOrInstallJava(j *Job) (string, error) {
	for _, p := range javaCandidates() {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if javaMajor(p) >= 21 {
			return p, nil
		}
	}
	j.set(func(s *JobStatus) { s.PhaseText = "Скачиваю Java 21 для установки NeoForge…" })
	j.logf("Java 21 не найдена, скачиваю Temurin JRE 21")
	osName, ext, bin := "windows", ".zip", "java.exe"
	if !isWindows() {
		osName, ext, bin = "linux", ".tar.gz", "java"
	}
	zipPath := filepath.Join(appDataDir(), "cache", "jre21"+ext)
	u := "https://api.adoptium.net/v3/binary/latest/21/ga/" + osName + "/x64/jre/hotspot/normal/eclipse?project=jdk"
	if err := simpleDownload(j, u, zipPath); err != nil {
		return "", fmt.Errorf("не удалось скачать Java: %w", err)
	}
	dst := filepath.Join(appDataDir(), "java")
	var uerr error
	if isWindows() {
		uerr = unzip(zipPath, dst)
	} else {
		uerr = untarGz(zipPath, dst)
	}
	if uerr != nil {
		return "", fmt.Errorf("не удалось распаковать Java: %w", uerr)
	}
	_ = os.Remove(zipPath)
	ms, _ := filepath.Glob(filepath.Join(dst, "*", "bin", bin))
	if len(ms) == 0 {
		return "", errors.New("Java распакована, но " + bin + " не найден")
	}
	return ms[0], nil
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	base, _ := filepath.Abs(dst)
	for _, f := range r.File {
		p := filepath.Join(base, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(p, base+string(os.PathSeparator)) {
			return fmt.Errorf("подозрительный путь в архиве: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(p, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(p)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func untarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	base, _ := filepath.Abs(dst)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		p := filepath.Join(base, filepath.FromSlash(h.Name))
		if !strings.HasPrefix(p, base+string(os.PathSeparator)) {
			return fmt.Errorf("подозрительный путь в архиве: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(p, 0o755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&0o755|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(h.Linkname) || strings.Contains(h.Linkname, "..") {
				continue
			}
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.Symlink(h.Linkname, p)
		}
	}
}
