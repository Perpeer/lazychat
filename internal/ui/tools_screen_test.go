package ui

import (
	"context"
	"testing"

	"lazychat/internal/core/agent"
)

// echoTool is an AI tool the registry does not list: only the Tool contract,
// no capability, to show what a new tool gets for nothing.
type echoTool struct{ bin string }

func (e echoTool) ID() string     { return "echo" }
func (e echoTool) Name() string   { return "Echo" }
func (e echoTool) Colour() string { return "#ff00ff" }
func (e echoTool) Start(dir, name string) agent.Exec {
	return agent.Exec{Dir: dir, Args: []string{e.bin}}
}
func (e echoTool) Check(context.Context) agent.Status {
	return agent.Status{Ready: true, Version: "1.2.3"}
}

// A tool added to the registry shows up everywhere tools are listed — the
// tools under Chat's tree, the new-session form, the Settings rows, doctor —
// and starts a session, with nothing outside its own type written for it.
func TestNewToolEverywhere(t *testing.T) {
	e, _ := seeded(t)
	e.tools = []agent.Tool{echoTool{bin: fake(t, "fake-codex.sh")}}
	d := start(t, e, 120, 32)
	d.expect("● echo    1.2.3")
	d.key("n")
	d.expect("create session", "AI tool")
	d.key("tab", "right", "right")
	d.expect("echo")
	d.key("tab")
	d.typ("hello")
	d.key("enter")
	d.expect("demo2 · hello · running")
	d.leave()
	d.expect("echo · ")
	d.tab(4)
	d.key("down", "enter")
	d.expect("○ echo")
	d.key("esc")
	found := false
	for _, c := range d.core.Doctor() {
		found = found || c.Name == "echo" && c.OK
	}
	if !found {
		t.Errorf("doctor does not check echo: %+v", d.core.Doctor())
	}
	d.quitApp()
}
