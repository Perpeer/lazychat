package git

import (
	"io/fs"
	"syscall"
)

// sfDataless is macOS's SF_DATALESS: a file whose content a file provider
// such as iCloud has not brought to this Mac.
const sfDataless = 0x40000000

func dataless(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Flags&sfDataless != 0
}
