package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// uninstallSeed is a Mac in a temp folder with everything install.sh and
// lazychat leave on one, and what must stay: a project, Claude Code's
// settings, iTerm keys of the user's own.
type uninstallSeed struct {
	home, prefix, tmp, plist string
	gone, kept               []string
}

func seedUninstall(t *testing.T) uninstallSeed {
	t.Helper()
	root := t.TempDir()
	s := uninstallSeed{home: filepath.Join(root, "home"), prefix: filepath.Join(root, "bin"), tmp: filepath.Join(root, "tmp")}
	put := func(path, body string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	s.gone = []string{
		put(filepath.Join(s.prefix, "lazychat"), "bin"),
		put(filepath.Join(s.home, "Applications", "LazychatBar.app", "Contents", "Info.plist"), "app"),
		put(filepath.Join(s.home, ".warp", "launch_configurations", "lazychat.yaml"), "yaml"),
		put(filepath.Join(s.tmp, "lazychat-notices-99999999-1", "k"), ""),
		put(filepath.Join(s.tmp, "lazychat-questions-1", "k"), ""),
	}
	s.gone[1] = filepath.Join(s.home, "Applications", "LazychatBar.app")
	s.kept = []string{
		put(filepath.Join(s.home, ".lazychat", "settings.json"), "{}"),
		put(filepath.Join(s.home, "code", "project", "main.go"), "package main"),
		put(filepath.Join(s.home, ".claude", "settings.json"), `{"statusLine":{}}`),
		put(filepath.Join(s.home, ".warp", "launch_configurations", "mine.yaml"), "mine"),
		put(filepath.Join(s.tmp, fmt.Sprintf("lazychat-notices-%d-1", os.Getpid()), "k"), ""),
	}
	// lazychat's ⌘1 and ⌘2, ⌘3 changed by the user, and a key of theirs.
	s.plist = put(filepath.Join(root, "iterm.plist"), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>GlobalKeyMap</key><dict>
<key>0x31-0x100000</key><dict><key>Action</key><integer>10</integer><key>Text</key><string>[49;9u</string></dict>
<key>0x32-0x100000</key><dict><key>Action</key><integer>10</integer><key>Text</key><string>[50;9u</string></dict>
<key>0x33-0x100000</key><dict><key>Action</key><integer>10</integer><key>Text</key><string>mine</string></dict>
<key>0x35-0x100000</key><dict><key>Action</key><integer>10</integer><key>Text</key><string>other</string></dict>
</dict></dict></plist>
`)
	return s
}

func (s uninstallSeed) run(t *testing.T, proc string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"../../uninstall.sh"}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+s.home, "PREFIX="+s.prefix, "TMPDIR="+s.tmp,
		"LAZYCHAT_UNINSTALL_TEST=1", "LAZYCHAT_ITERM_PLIST="+s.plist, "LAZYCHAT_PROC="+proc)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// A test binary started with LAZYFAKE_WAIT only waits, as a running
// lazychat would, until it is killed.
func init() {
	if os.Getenv("LAZYFAKE_WAIT") != "" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func (s uninstallSeed) itermKeys(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :GlobalKeyMap", s.plist).Output()
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, k := range []string{"0x31", "0x32", "0x33", "0x35"} {
		if strings.Contains(string(out), k+"-0x100000") {
			keys = append(keys, k)
		}
	}
	return strings.Join(keys, " ")
}

// uninstall.sh takes back the program, the menu bar app, Warp's launch
// configuration, dead lazychats' notices and the iTerm keys still
// lazychat's, and leaves the data, projects, Claude Code's settings, other
// keys and a running lazychat's notices; run again, it finds nothing left.
func TestUninstall(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	s := seedUninstall(t)
	out, err := s.run(t, "no-such-process")
	if err != nil {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}
	for _, p := range s.gone {
		if exists(p) {
			t.Errorf("still there: %s\n%s", p, out)
		}
	}
	for _, p := range s.kept {
		if !exists(p) {
			t.Errorf("removed: %s\n%s", p, out)
		}
	}
	if got := s.itermKeys(t); got != "0x33 0x35" {
		t.Errorf("iTerm keys left: %q, want the user's 0x33 0x35\n%s", got, out)
	}
	if !strings.Contains(out, "--purge moves it to the Trash") {
		t.Errorf("the data's line is missing:\n%s", out)
	}
	if out, err := s.run(t, "no-such-process"); err != nil || strings.Contains(out, "ok    removed") {
		t.Errorf("a second run: %v\n%s", err, out)
	}
}

// --dry-run changes nothing; --purge moves lazychat's data to the Trash.
func TestUninstallDryRunAndPurge(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	s := seedUninstall(t)
	if out, err := s.run(t, "no-such-process", "--dry-run", "--purge"); err != nil || !strings.Contains(out, "would trash") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	for _, p := range append(s.gone, s.kept...) {
		if !exists(p) {
			t.Errorf("a dry run removed %s", p)
		}
	}
	if got := s.itermKeys(t); got != "0x31 0x32 0x33 0x35" {
		t.Errorf("a dry run changed the iTerm keys: %q", got)
	}
	out, err := s.run(t, "no-such-process", "--purge")
	if err != nil {
		t.Fatalf("purge: %v\n%s", err, out)
	}
	if exists(filepath.Join(s.home, ".lazychat")) {
		t.Errorf("~/.lazychat is still there\n%s", out)
	}
	trashed, _ := filepath.Glob(filepath.Join(s.home, ".Trash", "lazychat-*", "settings.json"))
	if len(trashed) != 1 {
		t.Errorf("the data is not in the Trash: %v\n%s", trashed, out)
	}
}

// With lazychat running, uninstall.sh stops and removes nothing: quitting
// it would end its sessions.
func TestUninstallRefusesWhileRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	s := seedUninstall(t)
	// A copy of this test binary named lazyfake, which only waits (init
	// below): macOS will not run a moved copy of /bin/sleep.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "lazyfake")
	b, err := os.ReadFile(self)
	if err != nil || os.WriteFile(fake, b, 0o755) != nil {
		t.Fatalf("copy the test binary: %v", err)
	}
	p := exec.Command(fake)
	p.Env = append(os.Environ(), "LAZYFAKE_WAIT=1")
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Process.Kill(); _ = p.Wait() })
	time.Sleep(200 * time.Millisecond) // ps lists it once it has started
	out, err := s.run(t, "lazyfake")
	if err == nil || !strings.Contains(out, "quit it (q)") {
		t.Fatalf("did not stop: %v\n%s", err, out)
	}
	for _, p := range s.gone {
		if !exists(p) {
			t.Errorf("removed while lazychat ran: %s", p)
		}
	}
}
