package git

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
)

// downloadedMsg is an iCloud download done off the loop for a row.
type downloadedMsg struct {
	key string
	n   int
	err error
}

// inICloud is the files the cursor's row could not be read for, still in
// iCloud; nil when that is not why.
func (g *Git) inICloud() []string {
	p := g.cursorStatus()
	if p == nil {
		return nil
	}
	var cloud git.ErrInICloud
	if errors.As(p.err, &cloud) {
		return cloud.Files
	}
	return nil
}

// downloadICloud asks iCloud for the row's git files, off the loop: each
// can take a while, and the row is read again once they are in.
func (g *Git) downloadICloud() tea.Cmd {
	files := g.inICloud()
	r, ok := g.cursorRow()
	if len(files) == 0 || !ok {
		return nil
	}
	g.screen.Note("asking iCloud for %d files…", len(files))
	key := r.key
	return g.own(func() tea.Msg { return downloadedMsg{key: key, n: len(files), err: git.Download(files)} })
}

// downloaded says how the download went and reads the row again.
func (g *Git) downloaded(msg downloadedMsg) tea.Cmd {
	if msg.err != nil {
		g.screen.Note("iCloud: %v", msg.err)
	} else {
		g.screen.Note("iCloud brought %d files; reading again", msg.n)
	}
	return g.load(msg.key)
}
