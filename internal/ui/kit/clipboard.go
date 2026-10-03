package kit

import (
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
)

// CopyToClipboard puts text on the system clipboard: pbcopy on macOS, else an
// OSC 52 request, which iTerm and most modern terminals honour. A variable, so
// the screen tests record what is copied instead of taking the clipboard.
var CopyToClipboard = copySystem

func copySystem(text string) error {
	if path, err := exec.LookPath("pbcopy"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	_, err := os.Stdout.WriteString("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07")
	return err
}
