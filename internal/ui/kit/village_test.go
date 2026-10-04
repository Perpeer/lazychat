package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func villageText(v Village, frame int) string {
	rows := DrawVillage(v, frame, 96)
	for i := range rows {
		rows[i] = ansi.Strip(rows[i])
	}
	return strings.Join(rows, "\n")
}

// An empty village is Lazy and eight empty plots; workers fill the plots
// clockwise, the rest counted, never drawn.
func TestVillagePlots(t *testing.T) {
	empty := villageText(Village{}, 0)
	if n := strings.Count(empty, "· · · ·"); n != Plots || !strings.Contains(empty, "Lazy") || !strings.Contains(empty, "^^") {
		t.Errorf("empty village (%d plots):\n%s", n, empty)
	}
	v := Village{More: 3}
	for range Plots {
		v.Workers = append(v.Workers, Worker{Building: Hut, Title: "hut", Phase: Idle})
	}
	full := villageText(v, 0)
	if strings.Count(full, "· · · ·") != 0 || strings.Count(full, "└──────┘") != Plots || !strings.Contains(full, "+3") {
		t.Errorf("full village:\n%s", full)
	}
	if got := len(DrawVillage(v, 0, 96)); got != VillageRows {
		t.Errorf("%d rows, want %d", got, VillageRows)
	}
}

// A worker at work says its job over its building; one walking carries its
// bubble along; one done shows ✓; Lazy's rows follow its mood.
func TestVillageWorkers(t *testing.T) {
	work := villageText(Village{Leader: LeaderWorking, Workers: []Worker{{Building: Tower, Title: "Explore", Say: "find the brushes", Phase: AtWork}}}, 1)
	if !strings.Contains(work, "‹Explore: find the brushes›") || !strings.Contains(work, "▪") || !strings.Contains(work, "┬┬") {
		t.Errorf("at work:\n%s", work)
	}
	walk := villageText(Village{Workers: []Worker{{Building: Forge, Title: "forge", Say: "boards", Phase: Out, Progress: 0.5}}}, 0)
	if !strings.Contains(walk, "‹forge: boards›") {
		t.Errorf("walking:\n%s", walk)
	}
	back := villageText(Village{Workers: []Worker{{Building: Market, Title: "paint-shop", Phase: Return, Progress: 0.5}}}, 0)
	if !strings.Contains(back, "‹paint-shop ✓›") {
		t.Errorf("walking back:\n%s", back)
	}
	done := villageText(Village{Leader: LeaderParty, Workers: []Worker{{Building: Scribe, Title: "brush-care", Phase: Done}}}, 0)
	if !strings.Contains(done, "(^^)✓") || strings.Contains(done, "‹") || !strings.Contains(done, "^^") {
		t.Errorf("done:\n%s", done)
	}
	asking := villageText(Village{Leader: LeaderAsking}, 0)
	if !strings.Contains(asking, "?") {
		t.Errorf("asking:\n%s", asking)
	}
}
