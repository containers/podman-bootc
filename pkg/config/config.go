package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

const (
	projectName = "podman-bootc"
	configDir   = ".config/osc"
	cacheDir    = ".cache/osc"

	RunPidFile       = "run.pid"
	OciArchiveOutput = "image-archive.tar"

	DiskImage          = "disk.raw"
	CiDataIso          = "cidata.iso"
	CiDefaultTransport = "cdrom"

	CfgFile = "bc.cfg"
)

// VM files
//const (
//	runConfigFile  = "run.json"
//	installPidFile = "install.pid"
//	configFile     = "vm.json"
//	diskImage      = "disk.qcow2"
//
//	BootcOciArchive  = "image-archive.tar"
//	BootcOciDir      = "image-dir"
//	BootcCiDataDir   = "cidata"
//	BootcSshKeyFile  = "sshkey"
//	BootcSshPortFile = "sshport"
//)

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

var (
	User, _   = user.Current()
	SshDir    = filepath.Join(User.HomeDir, ".ssh")
	ConfigDir = filepath.Join(User.HomeDir, configDir)

	CacheDir        = filepath.Join(User.HomeDir, cacheDir)
	MachineImage    = filepath.Join(CacheDir, "machine/image.qcow2")
	MachineIdentity = filepath.Join(User.HomeDir, ".ssh", "podman-machine-default")
	DefaultIdentity = filepath.Join(User.HomeDir, ".ssh", "id_rsa")
)

func RunDir() string {
	return filepath.Join(xdg.RuntimeDir, projectName, "run")
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

type BcVmConfig struct {
	SshPort     int    `json:"SshPort"`
	SshIdentity string `json:"SshPriKey"`
}
