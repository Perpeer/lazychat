package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This run's notices folder carries its pid and goes when lazychat leaves;
// a later start clears the folders of lazychats that are gone and those of
// older builds, and keeps a running one's.
func TestNoticeFolders(t *testing.T) {
	a := &Actions{}
	f := a.questionFile("k")
	dir := filepath.Dir(f)
	if !strings.HasPrefix(filepath.Base(dir), fmt.Sprintf("%s%d-", noticePrefix, os.Getpid())) {
		t.Fatalf("the folder %q does not name this pid", dir)
	}
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Asked("k"); !ok {
		t.Error("a notice file is not a question")
	}
	a.dropNotices()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}

	tmp := t.TempDir()
	mine := filepath.Join(tmp, fmt.Sprintf("%s%d-1", noticePrefix, os.Getpid()))
	gone := filepath.Join(tmp, noticePrefix+"99999999-1")
	old := filepath.Join(tmp, "lazychat-questions-123")
	other := filepath.Join(tmp, "something-else")
	for _, d := range []string{mine, gone, old, other} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	clearStaleNotices(tmp)
	for d, want := range map[string]bool{mine: true, gone: false, old: false, other: true} {
		if _, err := os.Stat(d); (err == nil) != want {
			t.Errorf("%s kept = %v, want %v", filepath.Base(d), err == nil, want)
		}
	}
}
