package user

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"podman-bootc/pkg/config"
	"strconv"

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

func (u *User) MachineSocketDir() string {
	return filepath.Join(u.HomeDir(), ".local/share/containers/podman/machine/qemu")
}

func (u *User) MachineSocket() string {
	return filepath.Join(u.MachineSocketDir(), "podman.sock")
}

func (u *User) MachineSshKeyPriv() string {
	return filepath.Join(u.SSHDir(), "podman-machine-default")
}

func (u *User) MachineSshKeyPub() string {
	return filepath.Join(u.SSHDir(), "podman-machine-default.pub")
}

func (u *User) ConfigDir() string {
	return filepath.Join(u.HomeDir(), config.ConfigDir)
}

func (u *User) CacheDir() string {
	return filepath.Join(u.HomeDir(), config.CacheDir, config.ProjectName)
}

func (u *User) DefaultIdentity() string {
	return filepath.Join(u.SSHDir(), "id_rsa")
}

func (u *User) RunDir() string {
	return filepath.Join(xdg.RuntimeDir, config.ProjectName, "run")
}


func (u *User) InitOSCDirs() error {
	if err := os.MkdirAll(u.ConfigDir(), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(u.CacheDir(), os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(u.RunDir(), os.ModePerm); err != nil {
		return err
	}

	return nil
}

func (u *User) RemoveOSCDirs() error {
	if err := os.RemoveAll(u.ConfigDir()); err != nil {
		return err
	}
	if err := os.RemoveAll(u.CacheDir()); err != nil {
		return err
	}

	if err := os.RemoveAll(u.RunDir()); err != nil {
		return err
	}

	return nil
}
