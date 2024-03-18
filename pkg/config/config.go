package config

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/adrg/xdg"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/sirupsen/logrus"
)

const (
	projectName        = "podman-bootc"
	configDir          = ".config"
	cacheDir           = ".cache"
	RunPidFile         = "run.pid"
	OciArchiveOutput   = "image-archive.tar"
	DiskImage          = "disk.raw"
	CiDataIso          = "cidata.iso"
	SshKeyFile         = "sshkey"
	CfgFile            = "bc.cfg"
)

// the podman library switches to the root user when imported
// so we need to use rootless to get the correct user
func getUser() (u *user.User) {
	rootlessId := rootless.GetRootlessUID()

	var err error
	if rootlessId < 0 {
		u, err = user.Current()
	} else {
		u, err = user.LookupId(strconv.Itoa(rootlessId))
	}

	if err != nil {
		logrus.Errorf("failed to get user: %v", err)
		os.Exit(1)
	}

	return u
}

var (
	User              = getUser()
	UserSshDir        = filepath.Join(User.HomeDir, ".ssh")
	MachineSocket     = filepath.Join(User.HomeDir, ".local/share/containers/podman/machine/qemu/podman.sock")
	MachineSshKeyPriv = filepath.Join(UserSshDir, "podman-machine-default")
	MachineSshKeyPub  = filepath.Join(UserSshDir, "podman-machine-default.pub")
	ConfigDir         = filepath.Join(User.HomeDir, configDir)
	CacheDir          = filepath.Join(User.HomeDir, cacheDir, projectName)
	RunDir            = filepath.Join(xdg.RuntimeDir, projectName, "run")
	MachineCacheDir   = filepath.Join("/home/core", cacheDir, projectName)
	DefaultIdentity   = filepath.Join(UserSshDir, "id_rsa")
)
