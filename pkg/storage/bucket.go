package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman-bootc/pkg/define"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

var ErrInUse = errors.New("busy bucket")
var ErrFileNotFound = errors.New("file not found")

type UnlockFunc func() error
type accessMode uint

const invalidGuard = "invalid guard"
const (
	exclusive accessMode = iota
	shared
)

func NewBucket(cacheDir, runDir string) *Bucket {
	return &Bucket{
		cacheDir: cacheDir,
		runDir:   runDir,
	}
}

type Bucket struct {
	cacheDir string
	runDir   string
}

func (p *Bucket) SearchByPrefix(prefix string) (*define.FullImageId, error) {
	ids, err := p.List()
	if err != nil {
		return nil, err
	}

	for _, cachedId := range ids {
		if strings.HasPrefix(string(cachedId), prefix) {
			return &cachedId, nil
		}
	}

	return nil, nil
}

func (p *Bucket) List() ([]define.FullImageId, error) {
	entries, err := os.ReadDir(p.cacheDir)
	if err != nil {
		return nil, err
	}

	var ids []define.FullImageId
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 64 {
			ids = append(ids, define.FullImageId(e.Name()))
		}
	}

	return ids, nil
}

// Get returns a read-only guard and its unlock function to the bucket
// or nil if it does not exist. If it's unable to acquire the lock it returns
// an error
func (p *Bucket) Get(id define.FullImageId) (*ReadOnlyGuard, UnlockFunc, error) {
	// Before checking if the bucket exist let's acquire a lock to  make sure
	// that is not removed after checking it
	lock, err := p.lock(id, shared)
	if err != nil {
		return nil, nil, err
	}

	if !p.existsWithLock(id) {
		if err := lock.Unlock(); err != nil {
			logrus.Errorf("failed to unlock missing bucket: %s", id)
		}
		return nil, nil, nil
	}

	bucket := p.bucketPath(id)
	guard, unlock := makeROGuard(bucket, lock)
	return guard, unlock, nil
}

// GetExclusive returns a write guard and its unlock function to the bucket
// or nil if it does not exist. If it's unable to acquire the lock it returns
// an error
func (p *Bucket) GetExclusive(id define.FullImageId) (*WriteGuard, UnlockFunc, error) {
	// Before checking if the bucket exist let's acquire a lock to  make sure
	// that is not removed after checking it
	lock, err := p.lock(id, exclusive)
	if err != nil {
		return nil, nil, err
	}

	if !p.existsWithLock(id) {
		if err := lock.Unlock(); err != nil {
			logrus.Errorf("failed to unlock missing bucket: %s", id)
		}
		return nil, nil, nil
	}

	bucket := p.bucketPath(id)
	guard, unlock := makeWGuard(bucket, lock)
	return guard, unlock, nil
}

// GetExclusiveOrAdd returns a write guard if the bucket exists, or it will create
// a new bucket if it does not exist returning a write guard. If it's unable
// to acquire the lock it returns an error
func (p *Bucket) GetExclusiveOrAdd(id define.FullImageId) (*WriteGuard, UnlockFunc, error) {
	lock, err := p.lock(id, exclusive)
	if err != nil {
		return nil, nil, err
	}

	bucket := p.bucketPath(id)
	if !p.existsWithLock(id) {
		if err := os.MkdirAll(bucket, os.ModePerm); err != nil {
			if err := lock.Unlock(); err != nil {
				logrus.Errorf("failed to unlock new bucket: %s", id)
			}
			return nil, nil, fmt.Errorf("error while making bucket directory: %w", err)
		}
	}

	guard, unlock := makeWGuard(bucket, lock)
	return guard, unlock, nil
}

func (p *Bucket) existsWithLock(id define.FullImageId) bool {
	_, err := os.Stat(filepath.Join(p.cacheDir, string(id)))
	return err == nil
}

func (p *Bucket) lock(id define.FullImageId, mode accessMode) (*flock.Flock, error) {
	lockFile := filepath.Join(p.runDir, string(id)+".lock")
	lock := flock.New(lockFile)

	var locked bool
	var err error
	if mode == exclusive {
		locked, err = lock.TryLock()
	} else {
		locked, err = lock.TryRLock()
	}

	if err != nil {
		return nil, err
	}

	if !locked {
		return nil, ErrInUse
	}

	return lock, nil
}

func (p *Bucket) bucketPath(id define.FullImageId) string {
	return filepath.Join(p.cacheDir, string(id))
}

func makeROGuard(bucket string, lock *flock.Flock) (*ReadOnlyGuard, UnlockFunc) {
	guard := &ReadOnlyGuard{
		lock:   lock,
		path:   bucket,
		locked: true,
	}
	unlock := &unlockGuard{
		lock:  lock,
		guard: guard,
	}
	return guard, func() error { return unlock.unlock() }
}

type ReadOnlyGuard struct {
	lock   *flock.Flock
	path   string
	locked bool
}

func (p *ReadOnlyGuard) Load(fileName string) ([]byte, error) {
	p.lockGuard()

	fullPath, found := checkAndGetFullPath(p.path, fileName)
	if !found {
		return nil, fmt.Errorf("loading file: %w", ErrFileNotFound)
	}

	data, err := os.ReadFile(fullPath)

	if err != nil {
		return nil, fmt.Errorf("loading file: %w", err)
	}
	return data, nil
}

func (p *ReadOnlyGuard) lockGuard() {
	if !p.locked {
		panic(invalidGuard)
	}
}

func (p *ReadOnlyGuard) unlocked() {
	p.locked = false
}

func makeWGuard(bucket string, lock *flock.Flock) (*WriteGuard, UnlockFunc) {
	guard := &WriteGuard{
		lock:   lock,
		path:   bucket,
		locked: true,
	}
	unlock := &unlockGuard{
		lock:  lock,
		guard: guard,
	}
	return guard, func() error { return unlock.unlock() }
}

type WriteGuard struct {
	lock   *flock.Flock
	path   string
	locked bool
}

// FilePath returns the path to a file (possibly) stored in the bucket,
// and a boolean indicating if it's present or not.
func (p *WriteGuard) FilePath(fileName string) (string, bool) {
	p.lockGuard()

	return checkAndGetFullPath(p.path, fileName)
}

func (p *WriteGuard) Store(fileName string, data []byte) error {
	p.lockGuard()

	fullPath := filepath.Join(p.path, fileName)
	err := os.WriteFile(fullPath, data, 0660)
	if err != nil {
		return fmt.Errorf("storing file: %w", err)
	}
	return nil
}

// MoveIntoRename imports (moves) fileName to the bucket with a provided name.
// If fileName already exists inside the bucket, MoveInto replaces it.
func (p *WriteGuard) MoveIntoRename(srcFullPath, newName string) error {
	p.lockGuard()

	dstFullPath := filepath.Join(p.path, newName)

	// this could fail with "invalid cross-device link", if src and destination
	// are on different devices. In our use case this is not relevant, since we
	// expect the scratch and storage space will be in the same device
	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		return fmt.Errorf("importing file: failed to move %s to %s: %w", srcFullPath, newName, err)
	}
	return nil
}

func (p *WriteGuard) Remove() error {
	p.lockGuard()
	return os.RemoveAll(p.path)
}

func (p *WriteGuard) lockGuard() {
	if !p.locked {
		panic(invalidGuard)
	}
}

func (p *WriteGuard) unlocked() {
	p.locked = false
}

type guardApi interface {
	unlocked()
}

type unlockGuard struct {
	lock  *flock.Flock
	guard guardApi
}

func (p *unlockGuard) unlock() error {
	p.guard.unlocked()
	return p.lock.Unlock()
}

func checkAndGetFullPath(path, fileName string) (string, bool) {
	fullPath := filepath.Join(path, fileName)
	_, err := os.Stat(fullPath)
	return fullPath, err == nil
}
