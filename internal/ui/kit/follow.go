package kit

// Follow keeps the project a tab's cursor is on the one the user was last
// on, in whichever tab: every tab that lists the projects calls Sync each
// time it is drawn, so on coming into view it moves to the project chosen
// elsewhere, and what it is on becomes the choice for the others.
type Follow struct{ seen string }

// Sync follows shared when another tab changed it since this one last saw
// it, with sel putting the cursor there; then shared is what this tab is
// on. current is the project under the tab's cursor, "" for none.
func (f *Follow) Sync(shared *string, current string, sel func(project string)) {
	if *shared != "" && *shared != f.seen && *shared != current {
		sel(*shared)
		f.seen = *shared
		return
	}
	if current != "" {
		*shared = current
	}
	f.seen = *shared
}
