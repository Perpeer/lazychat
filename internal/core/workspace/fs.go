package workspace

import (
	"os"

	"lazychat/internal/core/files"
)

// rename is os.Rename; a test replaces it to take the path a move across
// disks takes.
var rename = os.Rename

// moveItem renames a file or folder, copying it when it is on another disk.
func moveItem(src, dst string) error { return files.Move(rename, src, dst) }
