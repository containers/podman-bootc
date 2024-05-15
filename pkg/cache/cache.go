package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
	"gitlab.com/bootc-org/podman-bootc/pkg/utils"
)

func NewCache(imageId string, user user.User) Cache {
	return Cache{
		ImageId: imageId,
		User:    user,
	}
}

type Cache struct {
	User      user.User
	ImageId   string
	Directory string
	Created	  bool
}

// Create VM cache dir; one per oci bootc image
func (p *Cache) Create() (err error) {
	p.Directory = filepath.Join(p.User.CacheDir(), p.ImageId)
	lock := utils.NewCacheLock(p.User.RunDir(), p.Directory)
	locked, err := lock.TryLock(utils.Exclusive)
	if err != nil {
		return fmt.Errorf("error locking the VM cache path: %w", err)
	}
	if !locked {
		return fmt.Errorf("unable to lock the VM cache path")
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			logrus.Errorf("unable to unlock VM %s: %v", p.ImageId, err)
		}
	}()

	if err := os.MkdirAll(p.Directory, os.ModePerm); err != nil {
		return fmt.Errorf("error while making bootc disk directory: %w", err)
	}

	p.Created = true

	return
}

func (p *Cache) GetDirectory() string {
	if !p.Created {
		panic("cache not created")
	}
	return p.Directory
}

func (p *Cache) GetDiskPath() string {
	if !p.Created {
		panic("cache not created")
	}
	return filepath.Join(p.GetDirectory(), "disk.raw")
}
