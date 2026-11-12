package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

const downloadWorkers = 6

func downloadAll(j *Job, m *Manifest, dir string, files []PackFile, st *LocalState) error {
	if len(files) == 0 {
		return nil
	}
	var bytesDone atomic.Int64
	var done atomic.Int64
	var firstErr error
	var errMu sync.Mutex
	var stMu sync.Mutex
	ch := make(chan PackFile)
	var wg sync.WaitGroup

	// speed meter
	stopMeter := make(chan struct{})
	go func() {
		last := int64(0)
		lastT := time.Now()
		var speed float64
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stopMeter:
				return
			case now := <-t.C:
				b := bytesDone.Load()
				inst := float64(b-last) / now.Sub(lastT).Seconds()
				if speed == 0 {
					speed = inst
				} else {
					speed = speed*0.7 + inst*0.3
				}
				last, lastT = b, now
				j.set(func(s *JobStatus) {
					s.BytesDone = b
					s.Done = int(done.Load())
					if !s.Paused {
						s.Speed = speed
					} else {
						s.Speed = 0
					}
				})
			}
		}
	}()

	for w := 0; w < downloadWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range ch {
				if !j.waitIfPaused() {
					continue
				}
				j.set(func(s *JobStatus) { s.Current = filepath.Base(f.Path) })
				err := downloadOne(j, m, dir, f, &bytesDone)
				if err != nil {
					errMu.Lock()
					if firstErr == nil && !errors.Is(err, errCanceled) {
						firstErr = fmt.Errorf("%s: %w", f.Path, err)
					}
					errMu.Unlock()
					j.logf("Не удалось скачать %s: %v", f.Path, err)
					continue
				}
				if f.Policy != "once" {
					stMu.Lock()
					st.Files[f.Path] = strings.ToLower(f.SHA1)
					stMu.Unlock()
				}
				done.Add(1)
			}
		}()
	}
	for _, f := range files {
		if j.isCanceled() {
			break
		}
		errMu.Lock()
		stop := firstErr != nil
		errMu.Unlock()
		if stop {
			break
		}
		ch <- f
	}
	close(ch)
	wg.Wait()
	close(stopMeter)
	j.set(func(s *JobStatus) { s.BytesDone = bytesDone.Load(); s.Done = int(done.Load()) })
	if j.isCanceled() {
		return errCanceled
	}
	return firstErr
}

func downloadOne(j *Job, m *Manifest, dir string, f PackFile, counter *atomic.Int64) error {
	dst := filepath.Join(dir, filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	urls := f.urls(m)
	if len(urls) == 0 {
		return errors.New("нет ссылки на файл")
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		for _, u := range urls {
			got, err := fetchTo(j, u, dst+".part", f.SHA1, counter)
			if err == nil {
				if err := replaceFile(dst+".part", dst); err != nil {
					return err
				}
				return nil
			}
			counter.Add(-got)
			_ = os.Remove(dst + ".part")
			if errors.Is(err, errCanceled) {
				return err
			}
			lastErr = err
		}
		select {
		case <-j.cancel:
			return errCanceled
		case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
		}
	}
	return lastErr
}

// replaceFile moves src over dst; on Windows a running game can lock dst, so retry a little.
func replaceFile(src, dst string) error {
	var err error
	for i := 0; i < 5; i++ {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		_ = os.Remove(dst)
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("не удалось записать файл (закрой игру): %w", err)
}

type cancelReader struct {
	r   io.Reader
	j   *Job
	cnt *atomic.Int64
	n   int64
}

func (c *cancelReader) Read(p []byte) (int, error) {
	if !c.j.waitIfPaused() {
		return 0, errCanceled
	}
	n, err := c.r.Read(p)
	c.n += int64(n)
	c.cnt.Add(int64(n))
	return n, err
}

func fetchTo(j *Job, u, dst, wantSHA1 string, counter *atomic.Int64) (int64, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "VibeClubLauncher/"+LauncherVersion+" (vibeclub.mcmod.ru)")
	ctxDone := make(chan struct{})
	defer close(ctxDone)
	resp, err := netClient(30 * time.Minute).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	h := sha1.New()
	cr := &cancelReader{r: resp.Body, j: j, cnt: counter}
	_, err = io.Copy(io.MultiWriter(out, h), cr)
	cerr := out.Close()
	if err != nil {
		return cr.n, err
	}
	if cerr != nil {
		return cr.n, cerr
	}
	if wantSHA1 != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, wantSHA1) {
			return cr.n, fmt.Errorf("файл повреждён при загрузке (контрольная сумма не совпала)")
		}
	}
	return cr.n, nil
}

// simpleDownload is used for Java / NeoForge installer (no manifest entry).
func simpleDownload(j *Job, u, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	var c atomic.Int64
	var last error
	urls := append([]string{u}, githubMirrors(u)...)
	for attempt := 0; attempt < 3*len(urls); attempt++ {
		_, err := fetchTo(j, urls[attempt%len(urls)], dst+".part", "", &c)
		if err == nil {
			return replaceFile(dst+".part", dst)
		}
		_ = os.Remove(dst + ".part")
		if errors.Is(err, errCanceled) {
			return err
		}
		last = err
		time.Sleep(2 * time.Second)
	}
	return last
}
