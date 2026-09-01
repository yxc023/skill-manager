package skillmanager

import "os"

// statDir is a thin wrapper so git.go does not depend directly on os for testing seams.
func statDir(p string) (os.FileInfo, error) {
	return os.Stat(p)
}
