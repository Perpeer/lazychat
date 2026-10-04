// Package testenv keeps tests off the machine that runs them: each test
// binary gets a home, a temp folder and a Claude config of its own, and git
// runs without the user's global or system config. Every package with tests
// calls Main from its TestMain; TestEveryPackage checks that none forgets.
package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

// Main runs the package's tests in a fresh environment and exits with
// their status.
func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	// Under /tmp, as short as the system's own temp folder is long: screen
	// tests find paths printed in a pane, where a long one wraps.
	root, err := os.MkdirTemp("/tmp", "lazychat-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	keepGo()
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	for _, d := range []string{home, tmp} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			panic(err)
		}
	}
	for k, v := range map[string]string{
		"HOME":              home,
		"TMPDIR":            tmp,
		"CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude"),
		"XDG_CONFIG_HOME":   filepath.Join(home, ".config"),
		// The user's hooks, signing and aliases must not run in a test repo.
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		// No test asks GitHub for the newest release.
		"LAZYCHAT_NO_UPDATE_CHECK": "1",
	} {
		os.Setenv(k, v)
	}
	// A shell's git or lazychat settings never reach a test: GIT_DIR once
	// made tests write into this repository's own config.
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "LAZYCHAT_TRACE"} {
		os.Unsetenv(k)
	}
	// With no global config git has no identity; a made-up one, unless a
	// test gives its own.
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Garden Shed", "GIT_AUTHOR_EMAIL": "shed@example.invalid",
		"GIT_COMMITTER_NAME": "Garden Shed", "GIT_COMMITTER_EMAIL": "shed@example.invalid",
	} {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	return m.Run()
}

// keepGo pins Go's caches to where they are now, so a test that runs the go
// tool under the new home neither rebuilds nor downloads everything.
func keepGo() {
	if os.Getenv("GOCACHE") == "" {
		if c, err := os.UserCacheDir(); err == nil {
			os.Setenv("GOCACHE", filepath.Join(c, "go-build"))
		}
	}
	if os.Getenv("GOPATH") == "" {
		if h, err := os.UserHomeDir(); err == nil {
			os.Setenv("GOPATH", filepath.Join(h, "go"))
		}
	}
}
