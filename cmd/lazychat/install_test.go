package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installCheck runs ./install.sh --check on a Mac made of stand-ins: uname,
// sw_vers and go answer as given, and the tools named in hide look missing.
func installCheck(t *testing.T, osName, macOS, goVersion, toolchain, hide string) (string, error) {
	t.Helper()
	bin := t.TempDir()
	shim := func(name, body string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	shim("uname", `[ "$1" = -m ] && echo arm64 || echo `+osName)
	shim("sw_vers", "echo "+macOS)
	shim("sysctl", `case "$2" in hw.optional.arm64) echo 1 ;; *) echo 0 ;; esac`)
	if goVersion != "" {
		shim("go", `case "$2" in GOVERSION) echo `+goVersion+` ;; GOTOOLCHAIN) echo `+toolchain+` ;; esac`)
	} else {
		hide += " go"
	}
	cmd := exec.Command("bash", "../../install.sh", "--check")
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "LAZYCHAT_HIDE="+hide+" brew")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Every install failure says what is wrong and what to do; what is only
// missing for an extra is a note, and the install goes on.
func TestInstallCheck(t *testing.T) {
	cases := []struct {
		name, os, macOS, goVersion, toolchain, hide string
		fails                                       bool
		says                                        []string
	}{
		{"linux", "Linux", "", "go1.26.0", "auto", "", true, []string{"runs on macOS for now", "Linux support is planned"}},
		{"old macOS", "Darwin", "11.7", "go1.26.0", "auto", "", true, []string{"macOS 11.7 is too old", "Software Update"}},
		{"no go", "Darwin", "15.1", "", "", "", true, []string{"Go is not installed", "Apple silicon (ARM64) installer at https://go.dev/dl", "brew install go"}},
		{"go before 1.21", "Darwin", "15.1", "go1.20.3", "auto", "", true, []string{"go1.20.3 is too old", "brew upgrade go"}},
		{"go 1.24 local", "Darwin", "15.1", "go1.24.2", "local", "", true, []string{"GOTOOLCHAIN=local", "GOTOOLCHAIN=auto ./install.sh"}},
		{"go 1.24 fetches", "Darwin", "15.1", "go1.24.2", "auto", "", false, []string{"go1.24.2 will download Go 1.26", "ok    macOS       15.1"}},
		{"go 1.26", "Darwin", "15.1", "go1.26.1", "auto", "", false, []string{"ok    Go          go1.26.1"}},
		{"no tools", "Darwin", "15.1", "go1.26.1", "auto", "clang swiftc", false, []string{"builds without reading the keyboard layout", "Lazy will not be in the menu bar", "xcode-select --install"}},
		{"macOS 12", "Darwin", "12.7", "go1.26.1", "auto", "", false, []string{"menu bar app needs macOS 13"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := installCheck(t, c.os, c.macOS, c.goVersion, c.toolchain, c.hide)
			if (err != nil) != c.fails {
				t.Errorf("failed = %v, want %v:\n%s", err != nil, c.fails, out)
			}
			for _, s := range c.says {
				if !strings.Contains(out, s) {
					t.Errorf("does not say %q:\n%s", s, out)
				}
			}
		})
	}
}

// doctor's extras say where the menu bar app and jq are, or how to get
// them, and fail nothing.
func TestExtraChecks(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Lazychat.app")
	none := func(string) (string, error) { return "", exec.ErrNotFound }
	got := extraChecks([]string{filepath.Join(t.TempDir(), "Lazychat.app")}, none)
	if got[0].OK || !strings.Contains(got[0].Detail, "./install.sh builds it") || !got[0].Optional {
		t.Errorf("no app: %+v", got[0])
	}
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	got = extraChecks([]string{app}, func(string) (string, error) { return "/opt/bin/jq", nil })
	if !got[0].OK || got[0].Detail != app || !got[1].OK || got[1].Detail != "/opt/bin/jq" {
		t.Errorf("both there: %+v", got)
	}
}
