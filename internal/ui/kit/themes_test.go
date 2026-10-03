package kit

import "testing"

// Every theme names every colour: a missing one would draw in the
// terminal's default and vanish on some backgrounds.
func TestThemes(t *testing.T) {
	seen := map[string]bool{}
	for i, th := range Themes() {
		if th.Name == "" || seen[th.Name] {
			t.Fatalf("theme %d: name %q empty or twice", i, th.Name)
		}
		seen[th.Name] = true
		for name, c := range map[string]string{
			"accent": string(th.Accent), "cursor": string(th.Cursor), "onfill": string(th.OnFill), "busy": string(th.Busy),
			"removed": string(th.Removed), "removed word": string(th.RemovedWord), "added": string(th.Added), "added word": string(th.AddedWord),
			"conflict": string(th.Conflict), "modified": string(th.BadgeModified), "new": string(th.BadgeAdded),
			"deleted": string(th.BadgeRemoved), "renamed": string(th.BadgeRenamed),
		} {
			if c == "" {
				t.Errorf("%s: no %s colour", th.Name, name)
			}
		}
		if i > 0 && (th.Background == "" || th.Foreground == "") {
			t.Errorf("%s: no window colours", th.Name)
		}
	}
	if got := ThemeByName("Nord").Name; got != "Nord" {
		t.Fatalf("ThemeByName(Nord) = %q", got)
	}
	for _, name := range []string{"", "gone"} {
		if got := ThemeByName(name).Name; got != "Gruvbox" {
			t.Fatalf("ThemeByName(%q) = %q, want Gruvbox", name, got)
		}
	}
	if got := ThemeByName("Amber").Name; got != "Amber" {
		t.Fatalf("ThemeByName(Amber) = %q", got)
	}
}
