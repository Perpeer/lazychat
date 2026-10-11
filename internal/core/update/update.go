// Package update knows the newest lazychat release: it asks GitHub's
// public API at every start and every hour, keeps the answer in lazychat's
// folder with GitHub's ETag, and compares versions.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Commit  string    `json:"commit,omitempty"`
	ETag    string    `json:"etag,omitempty"`
	Checked time.Time `json:"checked"`
}

func (c *Checker) path() string { return filepath.Join(c.Home, "update.json") }

// Release is a published release: its version, like 1.0.3, and the commit
// its tag is on, "" when GitHub did not say.
type Release struct{ Version, Commit string }

// Latest is the newest release's version, like 1.0.3, asked of GitHub
// every time: with the kept ETag an unchanged release is a 304 and the kept
// answer, so a release is seen at the next question, never a window later.
// When GitHub cannot be reached the kept answer stands; with none it is an
// error, and a bad answer keeps nothing.
func (c *Checker) Latest(ctx context.Context) (string, error) {
	r, err := c.Release(ctx)
	return r.Version, err
}

// Release is the newest release as Latest asks for it, with its commit,
// which is asked once per release and kept beside it.
func (c *Checker) Release(ctx context.Context) (Release, error) {
	var kept cached
	if c.Home != "" {
		_, _ = files.LoadJSON(c.path(), &kept, files.SetAside)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return Release{}, err
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
			return Release{kept.Latest, kept.Commit}, nil
		}
		return Release{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified && kept.Latest != "" {
		// An answer kept before commits were kept gets its commit now.
		if kept.Commit == "" {
			if kept.Commit = c.commit(ctx, "v"+kept.Latest); kept.Commit != "" && c.Home != "" {
				_ = files.SaveJSON(c.path(), kept, 0o644)
			}
		}
		return Release{kept.Latest, kept.Commit}, nil
	}
	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var body struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	v := strings.TrimPrefix(body.Tag, "v")
	if _, ok := parse(v); !ok {
		return Release{}, errors.New("the newest release has no version like 1.0.3: " + body.Tag)
	}
	commit := kept.Commit
	if v != kept.Latest || commit == "" {
		commit = c.commit(ctx, body.Tag)
	}
	if c.Home != "" {
		_ = files.SaveJSON(c.path(), cached{Latest: v, Commit: commit, ETag: res.Header.Get("ETag"), Checked: c.Now()}, 0o644)
	}
	return Release{v, commit}, nil
}

// commit is the commit tag is on, "" when GitHub cannot say: then the
// version alone decides.
func (c *Checker) commit(ctx context.Context, tag string) string {
	url := strings.TrimSuffix(c.URL, "/releases/latest") + "/commits/" + tag
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "lazychat")
	req.Header.Set("Accept", "application/vnd.github.sha")
	res, err := c.Client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 100))
	sha := strings.TrimSpace(string(b))
	if err != nil || res.StatusCode != http.StatusOK || len(sha) != 40 {
		return ""
	}
	return sha
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

// Behind says r is a release the build lacks: newer than base, the release
// it stands on, unless it is the build's own commit. A source build
// installed before its release was tagged stands on the tag before it.
func Behind(base, version string, r Release) bool {
	if !Newer(base, r.Version) {
		return false
	}
	number, hash, _ := strings.Cut(version, " ")
	hash, _, _ = strings.Cut(hash, "-")
	return number != r.Version || len(hash) < 7 || !strings.HasPrefix(r.Commit, hash)
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
