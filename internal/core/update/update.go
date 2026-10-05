// Package update knows the newest lazychat release: it asks GitHub's
// public API at every start and every hour, keeps the answer in lazychat's
// folder with GitHub's ETag, and compares versions.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lazychat/internal/core/files"
)

// LatestURL is GitHub's newest published release of lazychat.
const LatestURL = "https://api.github.com/repos/Perpeer/lazychat/releases/latest"

// Every is how often a running lazychat asks again. Asking costs nothing
// while the release is unchanged: the kept ETag makes GitHub answer 304,
// which its hourly limit does not count.
const Every = time.Hour

// Checker asks for the newest release. Home is lazychat's folder, where the
// last answer and its ETag are kept for the next question.
type Checker struct {
	URL    string
	Home   string
	Client *http.Client
	Now    func() time.Time
}

// New is a checker of lazychat's releases keeping its answer under home;
// nil when LAZYCHAT_NO_UPDATE_CHECK is set, as the tests set it.
func New(home string) *Checker {
	if os.Getenv("LAZYCHAT_NO_UPDATE_CHECK") != "" {
		return nil
	}
	return &Checker{URL: LatestURL, Home: home, Client: &http.Client{Timeout: 10 * time.Second}, Now: time.Now}
}

type cached struct {
	Latest  string    `json:"latest"`
	ETag    string    `json:"etag,omitempty"`
	Checked time.Time `json:"checked"`
}

func (c *Checker) path() string { return filepath.Join(c.Home, "update.json") }

// Latest is the newest release's version, like 1.0.3, asked of GitHub
// every time: with the kept ETag an unchanged release is a 304 and the kept
// answer, so a release is seen at the next question, never a window later.
// When GitHub cannot be reached the kept answer stands; with none it is an
// error, and a bad answer keeps nothing.
func (c *Checker) Latest(ctx context.Context) (string, error) {
	var kept cached
	if c.Home != "" {
		_, _ = files.LoadJSON(c.path(), &kept, files.SetAside)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return "", err
	}
	// Nothing about this Mac or this build goes with the question.
	req.Header.Set("User-Agent", "lazychat")
	req.Header.Set("Accept", "application/vnd.github+json")
	if kept.ETag != "" && kept.Latest != "" {
		req.Header.Set("If-None-Match", kept.ETag)
	}
	res, err := c.Client.Do(req)
	if err != nil {
		if kept.Latest != "" {
			return kept.Latest, nil
		}
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified && kept.Latest != "" {
		return kept.Latest, nil
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered %s", res.Status)
	}
	var body struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	v := strings.TrimPrefix(body.Tag, "v")
	if _, ok := parse(v); !ok {
		return "", errors.New("the newest release has no version like 1.0.3: " + body.Tag)
	}
	if c.Home != "" {
		_ = files.SaveJSON(c.path(), cached{Latest: v, ETag: res.Header.Get("ETag"), Checked: c.Now()}, 0o644)
	}
	return v, nil
}

// parse is a version like 1.0.3 as numbers.
func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Newer says latest is a later release than have; false when either is not
// a version like 1.0.3.
func Newer(have, latest string) bool {
	h, ok1 := parse(have)
	l, ok2 := parse(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range h {
		if l[i] != h[i] {
			return l[i] > h[i]
		}
	}
	return false
}

// Base is the release a build stands on: a Homebrew build's own version, or
// the release a source build's checkout is past (stamped by install.sh as
// release); "" when it is neither.
func Base(version, release string) string {
	if _, ok := parse(release); ok {
		return strings.TrimPrefix(release, "v")
	}
	if _, ok := parse(version); ok {
		return version
	}
	return ""
}
