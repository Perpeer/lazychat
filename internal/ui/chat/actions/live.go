package actions

import "lazychat/internal/term"

// Live is the chat sessions' running terminals, by record key.
type Live = term.Registry

func newLive() *Live { return term.NewRegistry() }
