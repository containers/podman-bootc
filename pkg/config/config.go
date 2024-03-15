package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

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

// VM Status
const (
	Installing string = "Installing"
	Running           = "Running"
	Stopped           = "Stopped"
)

type RunVmConfig struct {
	SshPort uint64 `json:"SshPort"`
	VncPort uint64 `json:"VncPort"`
}

type VmConfig struct {
	Name       string `json:"Name"`
	Vcpu       uint64 `json:"VCPU"`
	Mem        uint64 `json:"Mem"`
	DiskSize   uint64 `json:"DiskSize"`
	DiskImage  string `json:"DiskImage"`
	RunPidFile string `json:"RunPidFile"`
	SshPriKey  string `json:"SshPriKey"`
}

type BcVmConfig struct {
	SshPort     int    `json:"SshPort"`
	SshIdentity string `json:"SshPriKey"`
}

func BootcImagePath(id string) (string, error) {
	files, err := os.ReadDir(CacheDir)
	if err != nil {
		return "", err
	}

	imageId := ""
	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(f.Name(), id) {
			imageId = f.Name()
		}
	}

	if imageId == "" {
		return "", fmt.Errorf("local installation '%s' does not exists", id)
	}

	return filepath.Join(CacheDir, imageId), nil
}

func LoadConfig(id string) (*BcVmConfig, error) {
	vmPath, err := BootcImagePath(id)
	if err != nil {
		return nil, err
	}

	cfgFile := filepath.Join(vmPath, CfgFile)
	fileContent, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	cfg := new(BcVmConfig)
	if err := json.Unmarshal(fileContent, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func WriteConfig(vmDir string, sshPort int, sshIdentity string) error {
	bcConfig := BcVmConfig{SshPort: sshPort, SshIdentity: sshIdentity}
	bcConfigMsh, err := json.Marshal(bcConfig)
	if err != nil {
		return fmt.Errorf("marshal config data: %w", err)
	}
	cfgFile := filepath.Join(vmDir, CfgFile)
	err = os.WriteFile(cfgFile, bcConfigMsh, 0660)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}
