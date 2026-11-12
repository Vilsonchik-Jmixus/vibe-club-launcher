package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// raw.githubusercontent.com is slow or blocked for some players (seen on a Steam Deck from
// Russia: "Client.Timeout exceeded while awaiting headers"). Everything we host on GitHub
// is therefore also fetched through jsDelivr, which serves the same repository files.
//
// For freshness the manifest is taken from jsDelivr at the exact commit of main (resolved
// via api.github.com); "@main" on jsDelivr may lag behind by hours.

var rawGitHub = regexp.MustCompile(`^https://raw\.githubusercontent\.com/([^/]+)/([^/]+)/([^/?]+)/([^?]*)`)

var jsDelivrHosts = []string{"cdn.jsdelivr.net", "fastly.jsdelivr.net"}

// packCommit is the commit sha the current manifest came from ("" if unknown).
var (
	packCommitMu sync.Mutex
	packCommit   string
)

func setPackCommit(s string) {
	packCommitMu.Lock()
	packCommit = s
	packCommitMu.Unlock()
}

func getPackCommit() string {
	packCommitMu.Lock()
	defer packCommitMu.Unlock()
	return packCommit
}

// githubMirrors returns jsDelivr copies of a raw.githubusercontent.com URL.
func githubMirrors(u string) []string {
	m := rawGitHub.FindStringSubmatch(u)
	if m == nil {
		return nil
	}
	ref := m[3]
	if c := getPackCommit(); c != "" && ref == "main" {
		ref = c
	}
	var out []string
	for _, h := range jsDelivrHosts {
		out = append(out, fmt.Sprintf("https://%s/gh/%s/%s@%s/%s", h, m[1], m[2], ref, m[4]))
	}
	return out
}

// netClient waits at most 15 s for the first response bytes but lets slow bodies finish.
func netClient(total time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 15 * time.Second
	tr.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	tr.TLSHandshakeTimeout = 10 * time.Second
	return &http.Client{Timeout: total, Transport: tr}
}

func httpGetBytes(u string, limit int64) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "VibeClubLauncher/"+LauncherVersion)
	if strings.HasPrefix(u, "https://api.github.com/") {
		req.Header.Set("Accept", "application/vnd.github.sha")
	}
	resp, err := netClient(60 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// shortHost makes error messages readable: "raw.githubusercontent.com: …".
func shortHost(u string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.Index(s, "/"); i > 0 {
		s = s[:i]
	}
	return s
}

// fetchManifestBytes tries GitHub first, then jsDelivr at the latest commit, then jsDelivr @main.
func fetchManifestBytes(u string) ([]byte, error) {
	var errs []string
	try := func(src string) []byte {
		b, err := httpGetBytes(src, 64<<20)
		if err != nil {
			errs = append(errs, shortHost(src)+": "+err.Error())
			appendLog("Список сборки недоступен через " + shortHost(src) + ": " + err.Error())
			return nil
		}
		return b
	}
	fu := u
	if !strings.Contains(fu, "?") {
		// raw.githubusercontent.com caches for ~5 minutes; a changing query makes a fresh push visible.
		fu += fmt.Sprintf("?t=%d", time.Now().Unix()/30)
	}
	if b := try(fu); b != nil {
		setPackCommit("")
		return b, nil
	}
	m := rawGitHub.FindStringSubmatch(u)
	if m == nil {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	owner, repo, ref, path := m[1], m[2], m[3], m[4]
	if sha, err := httpGetBytes(fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, ref), 256); err == nil {
		s := strings.TrimSpace(string(sha))
		if regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(s) {
			for _, h := range jsDelivrHosts {
				if b := try(fmt.Sprintf("https://%s/gh/%s/%s@%s/%s", h, owner, repo, s, path)); b != nil {
					setPackCommit(s)
					appendLog("Список сборки получен через зеркало jsDelivr (коммит " + s[:7] + ")")
					return b, nil
				}
			}
		}
	} else {
		errs = append(errs, "api.github.com: "+err.Error())
	}
	for _, h := range jsDelivrHosts {
		if b := try(fmt.Sprintf("https://%s/gh/%s/%s@%s/%s", h, owner, repo, ref, path)); b != nil {
			setPackCommit("")
			appendLog("Список сборки получен через зеркало jsDelivr (@" + ref + ", может отставать на несколько часов)")
			return b, nil
		}
	}
	return nil, fmt.Errorf("GitHub и зеркала недоступны — проверь интернет или VPN (%s)", strings.Join(errs, "; "))
}
