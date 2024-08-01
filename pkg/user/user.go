package user

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/containers/podman-bootc/pkg/config"

	"github.com/adrg/xdg"
	"github.com/containers/podman/v5/pkg/rootless"
)

type User struct {
	OSUser *user.User
}

func NewUser() (u User, err error) {
	rootlessId := rootless.GetRootlessUID()

	var osUser *user.User
	if rootlessId < 0 {
		osUser, err = user.Current()
	} else {
		osUser, err = user.LookupId(strconv.Itoa(rootlessId))
	}

	if err != nil {
		return u, fmt.Errorf("failed to get user: %w", err)
	}

	return User{
		OSUser: osUser,
	}, nil
}

func (u *User) HomeDir() string {
	return u.OSUser.HomeDir
}

func (u *User) Username() string {
	return u.OSUser.Username
}

func (u *User) SSHDir() string {
	return filepath.Join(u.HomeDir(), ".ssh")
}

func (u *User) CacheDir() string {
	return filepath.Join(u.HomeDir(), config.CacheDir, config.ProjectName)
}

func (u *User) DefaultIdentity() string {
	return filepath.Join(u.SSHDir(), "id_rsa")
}

func (u *User) RunDir() string {
	if runtime.GOOS == "darwin" {
		tmpDir, ok := os.LookupEnv("TMPDIR")
		if !ok {
			tmpDir = "/tmp"
		}

		return filepath.Join(tmpDir, config.ProjectName, "run")
	}

	return filepath.Join(xdg.RuntimeDir, config.ProjectName, "run")
}

func (u *User) InitOSCDirs() error {
	if err := os.MkdirAll(u.CacheDir(), os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(u.RunDir(), os.ModePerm); err != nil {
		return err
	}

	return nil
}

func (u *User) RemoveOSCDirs() error {
	if err := os.RemoveAll(u.CacheDir()); err != nil {
		return err
	}

	if err := os.RemoveAll(u.RunDir()); err != nil {
		return err
	}

	return nil
}
