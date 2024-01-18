package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
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

func InitOSCDirs() error {
	if err := os.MkdirAll(ConfigDir, os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(CacheDir, os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(RunDir, os.ModePerm); err != nil {
		return err
	}

	return nil
}

// VM files
const (
	runConfigFile  = "run.json"
	runPidFile     = "run.pid"
	installPidFile = "install.pid"
	configFile     = "vm.json"
	diskImage      = "disk.qcow2"
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

func NewVM(name string, vcpu, mem, diskSize uint64) VmConfig {
	vm := NewVMPartial(name)
	vm.Vcpu = vcpu
	vm.Mem = mem
	vm.DiskSize = diskSize
	return vm
}

func NewVMPartial(name string) VmConfig {
	return VmConfig{
		Name:       name,
		DiskImage:  filepath.Join(ConfigDir, name, diskImage),
		RunPidFile: filepath.Join(RunDir, name, runPidFile),
		SshPriKey:  filepath.Join(SshDir, name),
	}
}

func (vm VmConfig) ConfigDir() string {
	return filepath.Dir(vm.DiskImage)
}

func (vm VmConfig) RunDir() string {
	return filepath.Dir(vm.RunPidFile)
}

func (vm VmConfig) ConfigFile() string {
	return filepath.Join(vm.ConfigDir(), configFile)
}

func (vm VmConfig) RunConfigFile() string {
	return filepath.Join(vm.RunDir(), runConfigFile)
}

func (vm VmConfig) InstallPidFile() string {
	return filepath.Join(vm.RunDir(), installPidFile)
}

func (vm VmConfig) SshKeys() (string, string) {
	pubKey := vm.SshPriKey + ".pub"
	return pubKey, vm.SshPriKey
}

func (vm VmConfig) Status() string {
	installPidFile := vm.InstallPidFile()
	runPidfile := vm.RunPidFile

	if live, _ := isProcessAlive(installPidFile); live {
		return Installing
	}

	if live, _ := isProcessAlive(runPidfile); live {
		return Running
	}
	return Stopped
}
func fileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func isCreated(name string) (bool, error) {
	probeVm := NewVMPartial(name)
	return fileExist(probeVm.ConfigFile())
}

func LoadVmFromDisk(name string) (*VmConfig, error) {
	exist, err := isCreated(name)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, fmt.Errorf("VM '%s' does not exists", name)
	}

	probeVm := NewVMPartial(name)
	fileContent, err := os.ReadFile(probeVm.ConfigFile())
	if err != nil {
		return nil, err
	}

	vm := new(VmConfig)
	if err := json.Unmarshal(fileContent, vm); err != nil {
		return nil, err
	}
	return vm, nil
}

func LoadRunningVmFromDisk(name string) (*RunVmConfig, error) {
	exist, err := isCreated(name)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, fmt.Errorf("VM '%s' does not exists", name)
	}

	probeVm := NewVMPartial(name)
	if probeVm.Status() != Running {
		return nil, fmt.Errorf("VM %s' is not running, you need to start it first", name)
	}

	fileContent, err := os.ReadFile(probeVm.RunConfigFile())
	if err != nil {
		return nil, err
	}

	runningVm := new(RunVmConfig)
	if err := json.Unmarshal(fileContent, runningVm); err != nil {
		return nil, err
	}
	return runningVm, nil
}

func isProcessAlive(pidFile string) (bool, error) {
	pid, err := readPidFile(pidFile)
	if err != nil {
		return false, err
	}
	return isPidAlive(pid), nil
}

func readPidFile(pidFile string) (int, error) {
	if _, err := os.Stat(pidFile); err != nil {
		return -1, err
	}

	fileContent, err := os.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}
	pidStr := string(bytes.Trim(fileContent, "\n"))
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		return -1, err
	}
	return int(pid), nil
}

func isPidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	exists := false

	if err == nil {
		exists = true
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return exists, err
}
