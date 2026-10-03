package term

import (
	"errors"
	"time"
)

// ErrNoPaste is a program that never asked for pasted text, so what is
// typed and what is pasted cannot be told apart and nothing is written.
var ErrNoPaste = errors.New("the program never asked for pasted text")

// ErrEnded is a program that ended before the text could be written.
var ErrEnded = errors.New("the program ended")

// PasteWhenReady writes text as a bracketed paste once the program has
// asked for bracketed paste — at once for one already up, even while it is
// busy printing — so it lands in the input as text and nothing is
// submitted: sending it is the user's Enter. Past limit nothing is written;
// the channel says why.
func (s *Session) PasteWhenReady(text string, limit time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() {
		end := time.Now().Add(limit)
		for {
			switch {
			case !s.Alive():
				done <- ErrEnded
				return
			case s.pasteMode.Load():
				done <- s.Write([]byte("\x1b[200~" + text + "\x1b[201~"))
				return
			case time.Now().After(end):
				done <- ErrNoPaste
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	return done
}
