package config

import (
	"os/user"
	"path/filepath"

	"github.com/adrg/xdg"
)

const (
	configDir  = ".config/osc"
	cacheDir   = ".cache/osc"
	netInstDir = cacheDir + "/netinst"

	BootcDiskImage = "disk.img"
)

var (
	User, _   = user.Current()
	SshDir    = filepath.Join(User.HomeDir, ".ssh")
	ConfigDir = filepath.Join(User.HomeDir, configDir)
	IsoImage  = filepath.Join(User.HomeDir, netInstDir, "fedora-netinst.iso")
	Kernel    = filepath.Join(User.HomeDir, netInstDir, "vmlinuz")
	Initrd    = filepath.Join(User.HomeDir, netInstDir, "initrd.img")

	CacheDir        = filepath.Join(User.HomeDir, cacheDir)
	MachineImage    = filepath.Join(CacheDir, "machine/image.qcow2")
	MachineIdentity = filepath.Join(User.HomeDir, ".ssh", "podman-machine-default")
	DefaultIdentity = filepath.Join(User.HomeDir, ".ssh", "id_rsa")
)

func RunDir() string {
	return xdg.RuntimeDir + "/podmanbootc/run"
}
