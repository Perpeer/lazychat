package kit

import "lazychat/internal/ui/vm"

// List is the models' cursor, used by kit's pickers too.
type List = vm.List

// Scroller is a tree's offset that the wheel can move off the cursor.
type Scroller = vm.Scroller

// DrawBlocks lays blocks out from the scroll offset into avail rows.
func DrawBlocks(blocks [][]string, from, avail int) []string {
	var lines []string
	for i := from; i < len(blocks) && len(lines) < avail; i++ {
		for _, row := range blocks[i] {
			if len(lines) >= avail {
				break
			}
			lines = append(lines, row)
		}
	}
	return lines
}
