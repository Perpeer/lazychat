package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/state"
	"lazychat/internal/core/status"
	"slices"
)

// mascot is the mascot's rows on the rail, its top four, as text.
func (d *driver) mascot() string {
	rows := strings.Split(d.screen(), "\n")
	var out []string
	for _, r := range rows[wsRows:min(len(rows), wsRows+4)] {
		out = append(out, string([]rune(r)[:min(railW, len([]rune(r)))]))
	}
	return strings.Join(out, "\n")
}

// until pumps the screen until ok holds, failing with why after a while.
func (d *driver) until(why string, ok func() bool) {
	d.t.Helper()
	d.untilIn(waitFor, why, ok)
}

// untilIn is until with its own limit, for a step that waits on a slow
// program.
func (d *driver) untilIn(limit time.Duration, why string, ok func() bool) {
	d.t.Helper()
	for end := time.Now().Add(limit); !ok(); d.pump(20 * time.Millisecond) {
		if time.Now().After(end) {
			d.t.Fatalf("%s:\n%s", why, d.screen())
		}
	}
}

// holds checks ok stays true for a while, pumping the program meanwhile:
// news that must not end by itself.
func (d *driver) holds(span time.Duration, why string, ok func() bool) {
	d.t.Helper()
	for end := time.Now().Add(span); time.Now().Before(end); d.pump(20 * time.Millisecond) {
		if !ok() {
			d.t.Fatalf("%s:\n%s", why, d.screen())
		}
	}
}

// session starts a session named name in demo2 and leaves its pane.
func (d *driver) session(name, line string) {
	d.t.Helper()
	d.key("n", "tab", "tab")
	d.typ(name)
	d.key("enter")
	d.expect("FAKE CLAUDE READY", "(ctrl+q) back to lazychat")
	if line != "" {
		d.raw(line + "\r")
	}
	d.leave()
}

// The mascot stands at the rail's top, at rest while nothing works; it
// types while a session works, and once that one is done it parties and
// says it waits on the footer until the session is looked at. A click on
// it — from another tab, or from inside another session — opens the
// waiting session with the keys.
func TestMascot(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	if m := d.mascot(); !strings.Contains(m, "^^") || !strings.HasPrefix(m, "╭────╮") {
		t.Fatalf("the mascot at rest, at the rail's top:\n%s", m)
	}

	d.session("ivy", "work")
	d.expect("ivy waits")
	d.untilIn(3*waitFor, "the mascot does not party for the finished session", func() bool { return strings.Contains(d.mascot(), "✦") })

	d.tab(3) // from another tab
	d.mouse(tea.MouseMsg{X: 2, Y: wsRows + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	d.expect("-n ivy", "(ctrl+q) back to lazychat")
	// Looked at, ivy stops calling: the party ends.
	d.raw("work\r") // and a new turn, which it finishes while no one looks
	d.leave()
	d.untilIn(3*waitFor, "ivy's new turn brought no news", func() bool { return strings.Contains(d.screen(), "ivy waits") })

	// From inside another session, whose pane has the keys and the mouse.
	d.session("oak", "")
	d.key("enter")
	d.expect("-n oak", "ivy waits")
	d.focus.mu.Lock()
	mouse := d.focus.mouse
	d.focus.mu.Unlock()
	mouse(0, 3, wsRows+2, false) // one-based, as the terminal reports it
	mouse(0, 3, wsRows+2, true)
	d.expect("-n ivy", "(ctrl+q) back to lazychat")
	d.leave()
	d.until("ivy still calls once it was looked at", func() bool { return !strings.Contains(d.screen(), "ivy waits") })
	d.until("the party went on once ivy was looked at", func() bool { return !strings.Contains(d.mascot(), "✦") })
	d.quitApp()
}

// A question plays in the mascot's own frame, a "?" on its top edge,
// until it is answered: a click on the mascot only brings Chat forward,
// where the asking session blinks in the list; opening it ends nothing,
// an answer — the session works again — does.
func TestMascotQuestions(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("oak", "ask")
	d.tab(3)
	asks := func() bool { return strings.Contains(strings.Split(d.mascot(), "\n")[0], "?") }
	d.until("the question does not play in the mascot", asks)
	d.expect("oak asks")
	d.mouse(tea.MouseMsg{X: 2, Y: wsRows + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	d.expect("session: (enter) continue", "oak asks")
	d.expectNot("(ctrl+q) back to lazychat")
	d.until("oak does not blink in the list", func() bool { return strings.Contains(d.screen(), "? oak") })

	d.selectSession("oak")
	d.key("enter")
	d.expect("-n oak", "(ctrl+q) back to lazychat")
	d.holds(1500*time.Millisecond, "the question ended once oak was open", asks)
	d.raw("work\r") // the answer
	d.until("the question stayed once it was answered", func() bool { return !asks() && !strings.Contains(d.screen(), "oak asks") })
	d.leave()
	d.quitApp()
}

// While a session works the mascot types on a keyboard under its face;
// when it is done it parties in its own frame, a star going round it;
// nothing above it moves and the tabs stay put.
func TestMascotAnimations(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	chat := d.railRow("chat")
	if chat != wsRows+5 { // its name's row, under the mascot's four rows and the box's top edge
		t.Errorf("Chat's box name is on row %d, want %d", chat, wsRows+5)
	}
	d.session("ivy", "work")
	d.until("no keyboard while ivy works", func() bool { return strings.Contains(d.mascot(), "▪") })
	d.untilIn(3*waitFor, "no party once ivy was done", func() bool { return strings.Contains(d.mascot(), "✦") })
	if got := d.railRow("chat"); got != chat {
		t.Errorf("Chat's box moved from row %d to %d", chat, got)
	}
	d.quitApp()
}

// A question is called out in Chat too, not only while Chat is off screen.
func TestMascotNewsGoesOn(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("oak", "ask")
	asks := func() bool { return strings.Contains(strings.Split(d.mascot(), "\n")[0], "?") }
	d.tab(3)
	d.until("oak's question is not called out", asks)
	d.tab(1)
	d.holds(1500*time.Millisecond, "the call ended in Chat", asks)
	d.expect("oak asks")
	d.quitApp()
}

// A finished session's party goes on with Chat open, its name blinking in
// the list; new work in it brings the typing back, and looking at it once
// it is done again ends the party, its ✓ steady.
func TestMascotDoneFlow(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("ivy", "work")
	d.untilIn(3*waitFor, "no party once ivy was done", func() bool { return strings.Contains(d.mascot(), "✦") })
	d.holds(2500*time.Millisecond, "the party ended with Chat open", func() bool { return strings.Contains(d.mascot(), "✦") })
	d.until("ivy's name does not blink", func() bool { return strings.Contains(d.screen(), "✓ ivy") })
	d.until("ivy's name does not blink back", func() bool { return !strings.Contains(d.screen(), "✓ ivy") })

	// New work in it: the party gives way to typing.
	d.selectSession("ivy")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("work long\r")
	d.leave()
	d.until("no typing for ivy's new work", func() bool { return strings.Contains(d.mascot(), "▪") && !strings.Contains(d.mascot(), "✦") })
	// "work long" works four seconds, then the second's grace tells it done.
	d.untilIn(3*waitFor, "no party once ivy was done again", func() bool { return strings.Contains(d.mascot(), "✦") })

	// Opening it is looking at it: the party ends, its ✓ stays steady.
	d.selectSession("ivy")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.leave()
	d.until("the party went on once ivy was opened", func() bool { return strings.Contains(d.mascot(), "^^") && !strings.Contains(d.mascot(), "✦") })
	d.holds(1200*time.Millisecond, "ivy's ✓ does not stay steady", func() bool { return strings.Contains(d.screen(), "✓ ivy") })
	d.quitApp()
}

// A question claude draws is a question the moment it is drawn, not a
// finished answer: the mascot says the session asks, never parties and
// never says it waits.
func TestMascotQuestionOnScreen(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("ivy", "choose")
	d.tab(3)
	for end := time.Now().Add(waitFor); !strings.Contains(d.screen(), "ivy asks"); d.pump(20 * time.Millisecond) {
		if strings.Contains(d.screen(), "ivy waits") || strings.Contains(d.mascot(), "✦") {
			t.Fatalf("the question was taken for a finished answer:\n%s", d.screen())
		}
		if time.Now().After(end) {
			t.Fatalf("the question on screen was not seen:\n%s", d.screen())
		}
	}
	d.quitApp()
}

// The mascot wears one badge on its top edge per session at work: one for
// one, two for two; as they finish the badges go one by one.
func TestMascotMany(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	top := func() string { return strings.Split(d.mascot(), "\n")[0] }
	d.session("ivy", "work long")
	d.expect("ivy working")
	d.until("no badge for ivy", func() bool { return strings.HasPrefix(top(), "╭───●╮") })
	d.session("oak", "work long")
	d.expect("2 working")
	d.until("no second badge for oak", func() bool { return strings.HasPrefix(top(), "╭──●●╮") })
	d.untilIn(3*waitFor, "the badges did not go once both were done", func() bool { return !strings.Contains(top(), "●") && strings.Contains(d.screen(), "waits") })
	d.quitApp()
}

// When one session finishes while another still works, the mascot parties
// for a moment, the other's badge still on, then types on; when the last
// one finishes it parties until a new prompt.
func TestMascotCheer(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("oak", "work long")
	d.session("ivy", "work")
	party := func() bool { m := d.mascot(); return strings.Contains(m, "^^") && !strings.Contains(m, "▪") }
	d.untilIn(3*waitFor, "no cheer when ivy was done with oak at work", func() bool {
		return party() && strings.HasSuffix(strings.Split(d.mascot(), "\n")[0], "●╮") && strings.Count(strings.Split(d.mascot(), "\n")[0], "●") == 1 && strings.Contains(d.screen(), "oak working")
	})
	d.until("the cheer did not give way to typing for oak", func() bool {
		m := d.mascot()
		return strings.Contains(m, "▪") && strings.Contains(d.screen(), "oak working")
	})
	d.untilIn(3*waitFor, "no party once oak was done too", func() bool { return party() && !strings.Contains(d.mascot(), "●") })
	d.holds(2500*time.Millisecond, "the last party did not go on", party)
	d.quitApp()
}

// With two sessions: both done, both blink and the mascot parties. Looking
// at one stops its call — a steady ✓ — while the other's goes on. Given a
// prompt, the first works and the mascot types again with its badge, the
// footer naming the other one not looked at yet; a click on the mascot
// goes there.
func TestMascotTwoSessions(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("ivy", "work")
	d.session("oak", "work")
	d.untilIn(3*waitFor, "no party once both were done", func() bool { return strings.Contains(d.mascot(), "✦") && strings.Contains(d.screen(), "oak waits") })
	d.until("ivy does not blink", func() bool { return strings.Contains(d.screen(), "✓ ivy") })

	d.selectSession("ivy")
	d.key("enter")
	d.expect("-n ivy", "(ctrl+q) back to lazychat")
	d.leave()
	d.holds(1200*time.Millisecond, "ivy, looked at, does not keep a steady ✓", func() bool { return strings.Contains(d.screen(), "✓ ivy") })
	d.until("oak stopped blinking", func() bool { return !strings.Contains(d.screen(), "✓ oak") })
	d.until("oak does not blink back", func() bool { return strings.Contains(d.screen(), "✓ oak") })
	d.expect("oak waits")
	d.expectNot("ivy waits")

	// A prompt in ivy: typing comes back with ivy's badge; the footer names
	// oak, which still waits.
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("work long\r")
	d.leave()
	d.until("the mascot does not type with ivy's badge", func() bool {
		m := d.mascot()
		return strings.Contains(m, "▪") && strings.HasPrefix(strings.Split(m, "\n")[0], "╭───●╮")
	})
	d.expect("ivy working · oak waits")

	// The mascot's click goes to oak, the one not looked at.
	d.mouse(tea.MouseMsg{X: 2, Y: wsRows + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	d.expect("-n oak", "(ctrl+q) back to lazychat")
	d.leave()
	d.until("oak's news went on once it was looked at", func() bool { return !strings.Contains(d.screen(), "oak waits") })

	// Once ivy is done too, one click on its blinking row stops it.
	d.untilIn(3*waitFor, "ivy did not finish", func() bool { return strings.Contains(d.screen(), "ivy waits") })
	sc := d.screen()
	y := lineOf(sc, " ivy")
	row := strings.Split(sc, "\n")[y]
	d.click(utf8.RuneCountInString(row[:strings.Index(row, " ivy")+1]), y)
	d.until("one click on ivy's row did not stop its call", func() bool { return !strings.Contains(d.screen(), "ivy waits") })
	d.holds(1200*time.Millisecond, "ivy, clicked, does not keep a steady ✓", func() bool { return strings.Contains(d.screen(), "✓ ivy") })
	d.quitApp()
}

// While a session asks, the mascot's keyboard row is all "?", under the
// "?" on its top edge; once the question is gone, so is that row.
func TestMascotAskKeyboard(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.session("oak", "ask")
	d.tab(3)
	d.until("no ? keyboard while oak asks", func() bool {
		rows := strings.Split(d.mascot(), "\n")
		return len(rows) >= 4 && strings.Contains(rows[0], "?") && strings.Contains(rows[3], "[????]")
	})
	d.quitApp()
}

// A session that works before it was given anything — claude starting, a
// resume loading its conversation — has finished nothing: no party and no
// blinking name. Its first answer to the user is news as before.
func TestStartWorkIsNoNews(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	e.vars = map[string]string{"FAKE_CLAUDE_START_WORK": "1"}
	d := start(t, e, 120, 32)
	d.session("ivy", "")
	d.expect("FAKE CLAUDE READY")
	d.holds(3*time.Second, "starting up was taken for a finished answer", func() bool {
		return !strings.Contains(d.screen(), "ivy waits") && !strings.Contains(d.mascot(), "✦")
	})
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("work\r")
	d.leave()
	d.expect("ivy waits")
	d.quitApp()
}

// A session's row shows its turn's time: nothing before a prompt, ◷
// counting while it works, ⏸ held while a question is up and going on
// after the answer, the total once done, and from zero on the next prompt.
func TestTurnTime(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	row := func() string {
		for _, l := range strings.Split(d.screen(), "\n") {
			if i := strings.Index(l, "claude"); i >= 0 && strings.Contains(l, "│") {
				if seg := l[i:]; strings.Contains(seg, "◷") || strings.Contains(seg, "⏸") || strings.Contains(seg, "s ") || strings.HasPrefix(strings.TrimSpace(seg), "claude") {
					return seg
				}
			}
		}
		return ""
	}
	d.session("ivy", "")
	d.expect("FAKE CLAUDE READY")
	d.holds(1500*time.Millisecond, "a time before any prompt", func() bool {
		return !strings.Contains(d.screen(), "◷") && !strings.Contains(d.screen(), "⏸")
	})
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("choose long\r")
	d.leave()
	d.until("no ◷ while it works", func() bool { return strings.Contains(d.screen(), "◷") })
	d.until("no ⏸ while the question is up", func() bool { return strings.Contains(d.screen(), "⏸") })
	held := row()
	if secs(held) < 1 {
		t.Fatalf("no time spent before the question: %q", held)
	}
	d.holds(2*time.Second, "the time ran on while the question was up", func() bool { return row() == held })
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("answer\r")
	d.leave()
	d.until("the answer did not set it counting again", func() bool { return strings.Contains(d.screen(), "◷") })
	if secs(row()) < secs(held) {
		t.Fatalf("the answer started the turn over: held %q, then %q", held, row())
	}
	d.untilIn(3*waitFor, "the time did not stop when done", func() bool {
		return !strings.Contains(d.screen(), "◷") && !strings.Contains(d.screen(), "⏸") && strings.Contains(d.screen(), "ivy waits")
	})
	done := row()
	if !strings.Contains(done, "s") {
		t.Fatalf("no total once done: %q", done)
	}
	d.holds(1500*time.Millisecond, "the total moved after the turn ended", func() bool { return row() == done })
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("work long\r")
	d.leave()
	d.until("the new prompt did not start from zero", func() bool {
		r := row()
		return strings.Contains(r, "◷ 0s") || strings.Contains(r, "◷ 1s")
	})
	d.quitApp()
}

// secs reads the seconds of a row's turn time, "◷ 1m 05s" → 65.
func secs(row string) int {
	m := regexp.MustCompile(`(?:(\d+)m )?(\d+)s`).FindStringSubmatch(row)
	if m == nil {
		return -1
	}
	n, _ := strconv.Atoi(m[2])
	if m[1] != "" {
		mins, _ := strconv.Atoi(m[1])
		n += 60 * mins
	}
	return n
}

// Lazy is heard as the board changes: a session asking plays ask, one
// whose answer ends on an API error plays error, once.
func TestMascotSounds(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	var played []sound.Name
	d.app.play = func(n sound.Name) { played = append(played, n) }
	heard := func(n sound.Name) func() bool {
		return func() bool { return slices.Contains(played, n) }
	}
	d.session("oak", "ask")
	d.until("no sound for oak's question", heard(sound.Ask))
	d.session("pine", "fail")
	d.until("no sound for pine's failure", heard(sound.Error))
	d.pump(1200 * time.Millisecond)
	errors := 0
	for _, n := range played {
		if n == sound.Error {
			errors++
		}
	}
	if errors != 1 {
		t.Fatalf("the failure played %d times: %v", errors, played)
	}
	d.quitApp()
}

// i opens the inbox: the sessions waiting on you, asking first, finished
// after; Enter opens the one chosen. With two or more waiting a click on
// Lazy opens the inbox too; with nothing waiting i only says so.
func TestInbox(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	d.key("i")
	d.expect("nothing waits on you")
	d.session("pine", "answer")
	d.session("oak", "ask")
	d.until("oak does not ask yet", func() bool { return strings.Contains(d.screen(), "oak asks") })
	d.untilIn(3*waitFor, "pine has not finished yet", func() bool {
		st, _ := d.app.mascotState()
		for _, s := range st.Sessions {
			if s.Name == "pine" && s.State == status.Done {
				return true
			}
		}
		return false
	})
	d.key("i")
	d.expect("waiting on you", "oak", "asks · demo2", "pine", "finished · demo2")
	sc := d.screen()
	if lineOf(sc, "asks · demo2") > lineOf(sc, "finished · demo2") {
		t.Errorf("asking is not first:\n%s", sc)
	}
	d.key("enter")
	d.expect("-n oak", "(ctrl+q) back to lazychat")
	d.leave()
	d.app.openMascot()
	d.pump(0)
	d.expect("waiting on you")
	d.key("esc")
	d.quitApp()
}
