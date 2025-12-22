//go:build windows
// +build windows

package xmluimcp

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx     = kernel32.NewProc("LockFileEx")
	procUnlockFileEx   = kernel32.NewProc("UnlockFileEx")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// acquireFileLock acquires an exclusive file lock on Windows
// Returns a function to unlock the file, or an error
func acquireFileLock(lock *os.File) (func(), error) {
	handle := syscall.Handle(lock.Fd())

	// LockFileEx parameters:
	// HANDLE hFile,
	// DWORD dwFlags,
	// DWORD dwReserved,
	// DWORD nNumberOfBytesToLockLow,
	// DWORD nNumberOfBytesToLockHigh,
	// LPOVERLAPPED lpOverlapped
	var overlapped syscall.Overlapped
	ret, _, err := procLockFileEx.Call(
		uintptr(handle),
		uintptr(LOCKFILE_EXCLUSIVE_LOCK), // Exclusive lock, blocking
		0,                                 // Reserved
		0,                                 // Lock entire file (low DWORD)
		0,                                 // Lock entire file (high DWORD)
		uintptr(unsafe.Pointer(&overlapped)),
	)

	if ret == 0 {
		return nil, err
	}

	return func() {
		procUnlockFileEx.Call(
			uintptr(handle),
			0, // Reserved
			0, // Unlock entire file (low DWORD)
			0, // Unlock entire file (high DWORD)
			uintptr(unsafe.Pointer(&overlapped)),
		)
	}, nil
}










