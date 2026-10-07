package terminal

import (
	"strings"
	"testing"

	"lazychat/internal/ui/kit"
)

// Each footer row names what can be done there; nothing works that the two
// rows do not name but moving the cursor; every explained key is in the help.
func TestKeymap(t *testing.T) {
	for name, c := range map[string]struct {
		keys []binding
		want string
	}{
		"terminal":    {shellKeys, "enter continue · n new · s new ssh · e rename · m move · d delete · v copy · ? help"},
		"ssh":         {connKeys, "enter connect · n new · s new ssh · e edit · d delete · v copy · ? help"},
		"no terminal": {emptyRowKeys, "enter/n new · s new ssh · ? help"},
		"project":     {projectKeys, "shift+o open · shift+e edit · shift+m move · shift+d delete"},
		"none":        {emptyKeys, "o open · ? help"},
		"shell":       {termKeys, "ctrl+q back to lazychat · click the list: back there · drag select · copy · other keys go to the shell"},
	} {
		var parts []string
		for _, h := range kit.FooterHints(c.keys) {
			parts = append(parts, h.Key+" "+h.Does)
		}
		if got := strings.Join(parts, " · "); got != c.want {
			t.Errorf("%s footer\n got %q\nwant %q", name, got, c.want)
		}
		if hidden := kit.Unlisted(append(append([]binding(nil), c.keys...), projectKeys...)); len(hidden) > 0 {
			t.Errorf("%s: keys that work unnamed: %q", name, hidden)
		}
	}
	help := helpText()
	for _, b := range append(append(append(shellKeys, connKeys...), projectKeys...), copyKeys...) {
		if d := b.Does(); d != "" && !strings.Contains(help, d) {
			t.Errorf("help lacks %q", d)
		}
	}
}
