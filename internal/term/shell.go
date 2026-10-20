package term

import (
	"os"
	"os/exec"
)

// Shell is the shell a terminal runs: $SHELL, else zsh, else sh.
func Shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		if _, err := exec.LookPath(s); err == nil {
			return s
		}
	}
	for _, s := range []string{"/bin/zsh", "/bin/sh"} {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	return "sh"
}

// ShellArgv runs the shell as a login shell, so it reads the user's
// profile, without the variables that say it is in Terminal.app: there
// macOS's own profile would restore a Terminal window's saved session into
// it and print about it, and it is not in Terminal.app but in lazychat's
// pane.
func ShellArgv(program string) []string {
	return []string{"/usr/bin/env", "-u", "TERM_PROGRAM", "-u", "TERM_PROGRAM_VERSION", "-u", "TERM_SESSION_ID", program, "-l"}
}
