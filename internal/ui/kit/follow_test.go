package kit

import "testing"

// A tab follows the project chosen in another, and its own choice becomes
// the shared one.
func TestFollow(t *testing.T) {
	shared := ""
	var a, b Follow
	aOn, bOn := "app", "web"
	a.Sync(&shared, aOn, func(p string) { aOn = p })
	if shared != "app" {
		t.Fatalf("shared %q after the first tab", shared)
	}
	b.Sync(&shared, bOn, func(p string) { bOn = p })
	if bOn != "app" || shared != "app" {
		t.Errorf("the second tab is on %q, shared %q", bOn, shared)
	}
	bOn = "web" // the user moves in the second tab
	b.Sync(&shared, bOn, func(p string) { bOn = p })
	a.Sync(&shared, aOn, func(p string) { aOn = p })
	if aOn != "web" || shared != "web" {
		t.Errorf("the first tab is on %q, shared %q", aOn, shared)
	}
	a.Sync(&shared, "", func(string) { t.Error("nothing to follow") }) // no project under the cursor
	if shared != "web" {
		t.Errorf("a tab on no project changed the choice to %q", shared)
	}
}
