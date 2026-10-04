package kit

import "lazychat/internal/core/sound"

// Beat is the shell's fast beat, numbered, sent to the active tab while it
// says it animates; it turns what moves on the page.
type Beat struct{ N int }

// PlaySound asks the shell for one of Lazy's sounds; the shell plays it
// unless the user turned sounds off.
type PlaySound struct{ Name sound.Name }

// Animator is a tab that wants the fast beat while it shows motion.
type Animator interface {
	Animating() bool
}
