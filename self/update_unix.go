//go:build !windows

package self

import (
	"os"

	"github.com/napalu/gosafedate/metadata"
)

// MaybeRunUpdateHelper is a no-op on non-Windows platforms.
// It exists so callers can invoke it unconditionally in main().
func MaybeRunUpdateHelper(_ []byte) {}

func replaceBinary(_ Config, oldPath, newPath string, _ *metadata.Metadata) error {
	return rename(newPath, oldPath)
}

func restartBinary(path string) error {
	return restart(path)
}

func restart(currPath string) error {
	return execSelf(currPath, os.Args, os.Environ())
}
