package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/bootc-org/podman-bootc/pkg/user"
)

// NewCache creates a new cache object
// Parameters:
//   - id: the full image ID
//   - user: the user who is running the podman-bootc command
func NewCache(id string, user user.User) (cache Cache, err error) {
	return Cache{
		ImageId:   id,
		User:      user,
		Directory: filepath.Join(user.CacheDir(), id),
	}, nil
}

type Cache struct {
	User      user.User
	ImageId   string
	Directory string
	lock      CacheLock
}

// Exists checks if the cache directory Exists
// returns false if any error occurs while checking
func (p *Cache) Exists() bool {
	_, err := os.Stat(p.Directory)
	return err == nil
}

// Create VM cache dir; one per oci bootc image
func (p *Cache) Create() (err error) {
	if err := os.MkdirAll(p.Directory, os.ModePerm); err != nil {
		return fmt.Errorf("error while making bootc disk directory: %w", err)
	}

	return
}

func (p *Cache) GetDirectory() string {
	if !p.Exists() {
		panic("cache does not exist")
	}
	return p.Directory
}

func (p *Cache) GetDiskPath() string {
	if !p.Exists() {
		panic("cache does not exist")
	}
	return filepath.Join(p.GetDirectory(), "disk.raw")
}

func (p *Cache) Lock(mode AccessMode) error {
	p.lock = NewCacheLock(p.User.RunDir(), p.Directory)
	locked, err := p.lock.TryLock(mode)
	if err != nil {
		return fmt.Errorf("error locking the cache path: %w", err)
	}
	if !locked {
		return fmt.Errorf("unable to lock the cache path")
	}

	return nil
}

func (p *Cache) Unlock() error {
	return p.lock.Unlock()
}
