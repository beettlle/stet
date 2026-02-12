//go:build unix

package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("session lock: flock: %w", err)
	}
	release = func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}
