package config

import (
	"os/user"
	"path/filepath"
)

const (
	runBaseDir = "/run/user"
	configDir  = ".config/osc"
	cacheDir   = ".cache/osc"
	netInstDir = cacheDir + "/netinst"
)

var (
	User, _   = user.Current()
	SshDir    = filepath.Join(User.HomeDir, ".ssh")
	ConfigDir = filepath.Join(User.HomeDir, configDir)
	RunDir    = filepath.Join(runBaseDir, User.Uid, "osc")
	IsoImage  = filepath.Join(User.HomeDir, netInstDir, "fedora-netinst.iso")
	Kernel    = filepath.Join(User.HomeDir, netInstDir, "vmlinuz")
	Initrd    = filepath.Join(User.HomeDir, netInstDir, "initrd.img")

	CacheDir        = filepath.Join(User.HomeDir, cacheDir)
	MachineImage    = filepath.Join(CacheDir, "machine/image.qcow2")
	MachineIdentity = filepath.Join(User.HomeDir, ".ssh", "podman-machine-default")
	DefaultIdentity = filepath.Join(User.HomeDir, ".ssh", "id_rsa")
)