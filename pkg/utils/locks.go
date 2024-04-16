package utils

import (
	"path/filepath"

	"github.com/gofrs/flock"
)

type AccessMode uint

const (
	Exclusive AccessMode = iota
	Shared
)

type CacheLock struct {
	inner *flock.Flock
}

// NewCacheLock  returns a new instance of *CacheLock. It takes the path to the VM cache dir.
func NewCacheLock(lockDir, cacheDir string) CacheLock {
	imageLongID := filepath.Base(cacheDir)
	cacheDirLockFile := filepath.Join(lockDir, imageLongID+".lock")
	return CacheLock{inner: flock.New(cacheDirLockFile)}
}

// TryLock takes an exclusive or shared lock, based on the parameter mode.
// The lock is non-blocking, if we are unable to lock the cache directory,
// the function will return false instead of waiting for the lock.
func (l CacheLock) TryLock(mode AccessMode) (bool, error) {
	if mode == Exclusive {
		return l.inner.TryLock()
	} else {
		return l.inner.TryRLock()
	}
}

// Unlock unlocks the cache lock.
func (l CacheLock) Unlock() error {
	return l.inner.Unlock()
}
