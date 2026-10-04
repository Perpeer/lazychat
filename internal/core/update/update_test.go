package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// The answer is GitHub's newest release, kept for Every; a bad answer is an
// error and keeps nothing; a kept answer spares the question.
func TestLatest(t *testing.T) {
	asked := 0
	body, status := `{"tag_name":"v1.0.3"}`, http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		if r.Header.Get("User-Agent") != "lazychat" {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
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
	before := asked
	now = now.Add(Every - time.Minute)
	if v, _ := c.Latest(ctx); v != "1.0.3" || asked != before {
		t.Errorf("a kept answer asked again: %q, %d asks", v, asked-before)
	}
	body = `{"tag_name":"v1.0.4"}`
	now = now.Add(2 * time.Minute)
	if v, _ := c.Latest(ctx); v != "1.0.4" || asked != before+1 {
		t.Errorf("an old answer was not refreshed: %q", v)
	}
	slow := &Checker{URL: srv.URL, Client: &http.Client{Timeout: time.Nanosecond}, Now: time.Now}
	if _, err := slow.Latest(ctx); err == nil {
		t.Error("a timeout gave a version")
	}
}
