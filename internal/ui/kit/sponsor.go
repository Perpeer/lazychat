package kit

import (
	"os/exec"
	"runtime"

	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/ui/text"
)

// SponsorZone marks the sponsor line for the click.
const SponsorZone = "sponsor"

// OpenURL opens a page in the user's browser: open on macOS, xdg-open
// elsewhere. A variable, so the screen tests record the page instead of
// opening it.
var OpenURL = openSystem

func openSystem(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Start()
}

// sponsorRow is the one line under the AI tools that asks for support.
func sponsorRow(w int) string {
	return zone.Mark(SponsorZone, text.Fit(" "+StyleAccent.Render("♥")+" sponsor lazychat", w))
}
