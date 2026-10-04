package settings

import (
	"testing"

	"lazychat/internal/core/api"
	coresettings "lazychat/internal/core/settings"
)

// Every setting is listed under a section, none left out by the grouping,
// none listed twice.
func TestSections(t *testing.T) {
	s := &Settings{core: &api.Core{Settings: &coresettings.Settings{}}}
	all, listed := s.all(), s.settings()
	if len(listed) != len(all) {
		t.Fatalf("%d settings, %d listed under sections", len(all), len(listed))
	}
	seen := map[string]bool{}
	for _, st := range listed {
		if st.section == "" || seen[st.name] {
			t.Errorf("%q: section %q, twice %v", st.name, st.section, seen[st.name])
		}
		seen[st.name] = true
	}
}
