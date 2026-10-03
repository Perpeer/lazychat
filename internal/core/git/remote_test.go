package git

import (
	"errors"
	"path/filepath"
	"testing"
)

// Against a bare remote: a branch with no upstream is told so and pushed
// with tracking; a second clone's commit makes the first one's push
// rejected; a fast-forward pull takes it; once both sides have commits the
// pull refuses to merge and a rebase pull lines them up.
func TestPushPull(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "remote.git")
	sh(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", bare)
	a := repo(t)
	sh(t, a, "remote", "add", "origin", bare)

	if err := Push(a, "main", false); !errors.As(err, new(ErrNoUpstream)) {
		t.Fatalf("push with no upstream: %v", err)
	}
	if err := Push(a, "main", true); err != nil {
		t.Fatalf("push and track: %v", err)
	}
	if st, _ := StatusOf(a); st.Upstream != "origin/main" {
		t.Fatalf("upstream %q", st.Upstream)
	}

	b := t.TempDir()
	sh(t, b, "clone", "-q", bare, ".")
	sh(t, b, "config", "user.email", "t@t")
	sh(t, b, "config", "user.name", "t")
	write(t, b, "from-b.txt", "b\n")
	sh(t, b, "add", ".")
	sh(t, b, "commit", "-qm", "from b")
	if err := Push(b, "main", false); err != nil {
		t.Fatalf("b's push: %v", err)
	}

	write(t, a, "from-a.txt", "a\n")
	sh(t, a, "add", ".")
	sh(t, a, "commit", "-qm", "from a")
	if err := Push(a, "main", false); !errors.Is(err, ErrRejected) {
		t.Fatalf("a's push behind the remote: %v", err)
	}
	if err := Pull(a, false); !errors.Is(err, ErrDiverged) {
		t.Fatalf("a fast-forward pull with both sides ahead: %v", err)
	}
	if err := Pull(a, true); err != nil {
		t.Fatalf("a rebase pull: %v", err)
	}
	if err := Push(a, "main", false); err != nil {
		t.Fatalf("a's push after the rebase: %v", err)
	}

	if err := Pull(b, false); err != nil {
		t.Fatalf("b's fast-forward pull: %v", err)
	}
	if st, _ := StatusOf(b); st.Ahead != 0 || st.Behind != 0 {
		t.Fatalf("b after the pull: ↑%d ↓%d", st.Ahead, st.Behind)
	}
}
