package s3fs

import (
	"io/fs"
)

func dirEntriesContains(entries []fs.DirEntry, name string) bool {
	for _, e := range entries {
		if e.Name() == name {
			return true
		}
	}

	return false
}
