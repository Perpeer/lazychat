package api

import (
	"time"

	"lazychat/internal/core/git"
)

// headFresh is how long a project's HEAD is taken as read: the Git tab's
// beat, so the branch a heading names follows a switch made elsewhere.
const headFresh = 3 * time.Second

type headRead struct {
	h  git.Head
	ok bool
	at time.Time
}

// Head is what the checkout in a project's folder is on, from its HEAD file;
// ok is false outside a repository. A tab may ask on every redraw: the file
// is read again only once headFresh has passed.
func (c *Core) Head(path string) (git.Head, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.heads[path]; ok && time.Since(r.at) < headFresh {
		return r.h, r.ok
	}
	h, ok := git.HeadOf(path)
	if c.heads == nil {
		c.heads = map[string]headRead{}
	}
	c.heads[path] = headRead{h, ok, time.Now()}
	return h, ok
}
