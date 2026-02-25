//go:build windows

package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	_lockFileExclusive                     = 2
	_lockFileFailImmediately               = 1
	_lockViolation           syscall.Errno = 0x21
)

var (
	_modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	_procLockFileEx   = _modkernel32.NewProc("LockFileEx")
	_procUnlockFileEx = _modkernel32.NewProc("UnlockFileEx")
)

// AcquireLock acquires an exclusive advisory lock under stateDir so only one
// active session exists per repo. Creates stateDir if needed. Non-blocking:
// if the lock is already held, returns ErrLocked. On success, returns a
// release function that the caller should defer.
func AcquireLock(stateDir string) (release func(), err error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("session lock: create state dir: %w", err)
	}
	path := filepath.Join(stateDir, lockFilename)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("session lock: open %s: %w", path, err)
	}
	handle := syscall.Handle(f.Fd())
	flags := _lockFileExclusive | _lockFileFailImmediately
	var overlapped syscall.Overlapped
	r1, _, err := _procLockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 == 0 {
		_ = f.Close()
		if err != nil && errors.Is(err, _lockViolation) {
			return nil, ErrLocked
		}
		if err == nil {
			err = errors.New("LockFileEx failed")
		}
		return nil, fmt.Errorf("session lock: LockFileEx: %w", err)
	}
	release = func() {
		var overlapped syscall.Overlapped
		_, _, _ = _procUnlockFileEx.Call(
			uintptr(handle),
			0,
			1,
			0,
			uintptr(unsafe.Pointer(&overlapped)),
		)
		_ = f.Close()
	}
	return release, nil
}
