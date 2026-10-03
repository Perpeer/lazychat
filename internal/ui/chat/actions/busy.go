package actions

import "lazychat/internal/core/state"

// busy offers what claude itself suggests when a session is open elsewhere:
// a fork, attaching to the background session, stopping it and resuming
// here, or a fresh session in the same project. The pane behind the popup
// still shows claude's message.
func (a *Actions) busy(key, sessionID, short string) {
	var rec state.Session
	for _, r := range a.core.Store.Sessions {
		if r.Key == key {
			rec = r
		}
	}
	p, ok := a.core.Store.ProjectNamed(rec.Project)
	if !ok {
		a.host.Note("%s is open elsewhere and its project %q is no longer registered", rec.Name, rec.Project)
		return
	}
	type option struct {
		label string
		run   func()
	}
	options := []option{
		{"fork — branch off a copy (--fork-session)", func() {
			e, err := a.core.Fork(p, rec.Tool, sessionID)
			var fork state.Session
			if err == nil {
				fork, err = a.core.Store.AddSession(rec.Tool, rec.Name+" (fork)", p.Name, "")
			}
			if err == nil {
				err = a.launch(e, p, fork) // its id is learned from sessions/<pid>.json
			}
			if err != nil {
				a.host.Note("fork: %v", err)
			}
		}},
	}
	if short != "" {
		options = append(options,
			option{"attach — open the running background session", func() {
				e, err := a.core.Attach(p, rec.Tool, short)
				if err == nil {
					err = a.launch(e, p, rec)
				}
				if err != nil {
					a.host.Note("attach: %v", err)
				}
			}},
			option{"stop it, then resume here", func() {
				if err := a.core.Stop(rec.Tool, short); err != nil {
					a.host.Note("%v", err)
					return
				}
				if err := a.launchResume(p, rec, sessionID); err != nil {
					a.host.Note("resume: %v", err)
				}
			}})
	}
	options = append(options, option{"create a session in " + p.Name, func() {
		a.host.SelectProject(p.Name)
		a.NewSession(p.Name)
	}})
	a.host.Pick(rec.Name+" is open elsewhere — how to go on?", len(options), 0, func(i int) Row { return Row{Text: options[i].label} }, func(i int) { options[i].run() })
}
