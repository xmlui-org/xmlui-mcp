//go:build !windows
// +build !windows

package xmluimcp

import (
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive file lock on Unix systems (macOS, Linux)
// Returns a function to unlock the file, or an error
func acquireFileLock(lock *os.File) (func(), error) {
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	return func() {
		syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}, nil
}









