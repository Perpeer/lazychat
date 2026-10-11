package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lazychat/internal/core/files"
	"lazychat/internal/core/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// Versions compare by major, then minor, then patch, as numbers.
func TestNewer(t *testing.T) {
	for _, c := range []struct {
		have, latest string
		want         bool
	}{
		{"1.0.1", "1.0.3", true}, {"1.0.9", "1.0.10", true}, {"1.0.10", "1.0.9", false},
		{"1.2.0", "1.10.0", true}, {"2.0.0", "1.9.9", false}, {"1.0.3", "1.0.3", false},
		{"1.0(58) 1626be3", "1.0.3", false}, {"1.0.1", "v1.0.2", true}, {"", "1.0.1", false},
	} {
		if got := Newer(c.have, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.have, c.latest, got)
		}
	}
	if Base("1.0.2", "") != "1.0.2" || Base("1.0(58) 1626be3", "v1.0.1") != "1.0.1" || Base("dev", "") != "" {
		t.Error("Base")
	}
	// A source build is stamped with the number it becomes and its hash;
	// it stands on the newest tag, so a release it has not become yet is
	// no newer release.
	if Base("1.0.3 d9f8a5b", "v1.0.2") != "1.0.2" || Base("1.0.3 d9f8a5b", "") != "" || Newer(Base("1.0.3 d9f8a5b", "v1.0.2"), "1.0.2") {
		t.Error("Base of a source build")
	}
}

// The answer is GitHub's newest release, asked every time with the kept
// ETag: an unchanged release is a 304 and the kept answer, a new one is seen
// at the next question; a bad answer is an error and keeps nothing; with
// GitHub out of reach the kept answer stands.
func TestLatest(t *testing.T) {
	asked, notModified := 0, 0
	body, status, etag := `{"tag_name":"v1.0.3"}`, http.StatusOK, `"one"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		if r.Header.Get("User-Agent") != "lazychat" {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		if status == http.StatusOK && r.Header.Get("If-None-Match") == etag {
			notModified++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := &Checker{URL: srv.URL, Home: t.TempDir(), Client: srv.Client(), Now: func() time.Time { return now }}
	ctx := context.Background()

	status = http.StatusNotFound
	if _, err := c.Latest(ctx); err == nil {
		t.Fatal("a 404 gave a version")
	}
	status, body = http.StatusOK, `not json`
	if _, err := c.Latest(ctx); err == nil {
		t.Fatal("garbage gave a version")
	}
	body = `{"tag_name":"nightly"}`
	if _, err := c.Latest(ctx); err == nil {
		t.Fatal("a tag that is no version gave one")
	}
	body = `{"tag_name":"v1.0.3"}`
	if v, err := c.Latest(ctx); err != nil || v != "1.0.3" {
		t.Fatalf("latest %q %v", v, err)
	}
	if v, _ := c.Latest(ctx); v != "1.0.3" || notModified != 1 {
		t.Errorf("an unchanged release was not a 304 with the kept ETag: %q, %d 304s", v, notModified)
	}
	// A release out a minute later is seen at once, with no window to wait.
	body, etag = `{"tag_name":"v1.0.4"}`, `"two"`
	now = now.Add(time.Minute)
	if v, _ := c.Latest(ctx); v != "1.0.4" {
		t.Errorf("a new release was not seen at the next question: %q", v)
	}
	srv.Close()
	if v, err := c.Latest(ctx); err != nil || v != "1.0.4" {
		t.Errorf("out of reach, the kept answer did not stand: %q %v", v, err)
	}
	slow := &Checker{URL: srv.URL, Client: &http.Client{Timeout: time.Nanosecond}, Now: time.Now}
	if _, err := slow.Latest(ctx); err == nil {
		t.Error("a timeout with nothing kept gave a version")
	}
	_ = asked
}

// A source build installed before its own release was tagged stands on the
// tag before, yet it is that release: the release's commit says so. A build
// of another commit with the same number is still behind.
func TestBehindSameNumber(t *testing.T) {
	commits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/commits/v1.0.18" {
			commits++
			if r.Header.Get("Accept") != "application/vnd.github.sha" {
				t.Errorf("accept %q", r.Header.Get("Accept"))
			}
			_, _ = w.Write([]byte("d4c1ea4d57035f2436075709e49aedd370b0a0d3"))
			return
		}
		w.Header().Set("ETag", `"one"`)
		if r.Header.Get("If-None-Match") == `"one"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.18"}`))
	}))
	defer srv.Close()
	c := &Checker{URL: srv.URL + "/releases/latest", Home: t.TempDir(), Client: srv.Client(), Now: time.Now}
	r, err := c.Release(context.Background())
	if err != nil || r.Version != "1.0.18" || r.Commit != "d4c1ea4d57035f2436075709e49aedd370b0a0d3" {
		t.Fatalf("release %+v %v", r, err)
	}
	if r2, _ := c.Release(context.Background()); r2 != r || commits != 1 {
		t.Errorf("an unchanged release asked for its commit again: %+v, %d asks", r2, commits)
	}
	// An answer kept with no commit, as an older lazychat kept it, gets one
	// at the next 304.
	if err := files.SaveJSON(c.path(), cached{Latest: "1.0.18", ETag: `"one"`}, 0o644); err != nil {
		t.Fatal(err)
	}
	if r3, _ := c.Release(context.Background()); r3 != r || commits != 2 {
		t.Errorf("a kept answer with no commit: %+v, %d asks", r3, commits)
	}
	base := Base("1.0.18 d4c1ea4", "v1.0.17")
	for _, b := range []struct {
		version string
		want    bool
	}{
		{"1.0.18 d4c1ea4", false}, {"1.0.18 d4c1ea4-dirty.6e006e6", false},
		{"1.0.18 aaaa111", true}, {"1.0.17 d4c1ea4", true},
	} {
		if got := Behind(base, b.version, r); got != b.want {
			t.Errorf("Behind(%q) = %v", b.version, got)
		}
	}
	if !Behind(base, "1.0.18 d4c1ea4", Release{Version: "1.0.18"}) {
		t.Error("with no commit known, the number alone decides")
	}
	if Behind("1.0.18", "1.0.18", Release{Version: "1.0.18"}) || !Behind("1.0.17", "1.0.17", Release{Version: "1.0.18"}) {
		t.Error("a Homebrew build")
	}
}
