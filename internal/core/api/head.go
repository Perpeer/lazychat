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
// ok is false outside a repository. A tab asks on every redraw, so only a
// folder's first answer reads the file there; once headFresh has passed the
// file is read again off the loop, the last answer standing until then.
func (c *Core) Head(path string) (git.Head, bool) {
	c.mu.Lock()
	r, known := c.heads[path]
	if known {
		if time.Since(r.at) >= headFresh {
			r.at = time.Now() // one read at a time per folder
			c.heads[path] = r
			go c.readHead(path)
		}
		c.mu.Unlock()
		return r.h, r.ok
	}
	c.mu.Unlock()
	return c.readHead(path)
}

func (c *Core) readHead(path string) (git.Head, bool) {
	h, ok := git.HeadOf(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.heads == nil {
		c.heads = map[string]headRead{}
	}
	c.heads[path] = headRead{h, ok, time.Now()}
	return h, ok
}
