//go:build !darwin

package git

import "io/fs"

// dataless is a macOS file provider's mark; elsewhere no file has it.
func dataless(fs.FileInfo) bool { return false }
