package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/sirupsen/logrus"
)

type BootcVMMac struct {
	socketFile string
	BootcVMCommon
}

func NewVM(params NewVMParameters) (vm *BootcVMMac, err error) {
	if params.ImageID == "" {
		return nil, fmt.Errorf("image ID is required")
	}

	longId, cacheDir, err := GetVMCachePath(params.ImageID, params.User)
	if err != nil {
		return nil, fmt.Errorf("unable to get VM cache path: %w", err)
	}

	lock, err := lockVM(params, cacheDir)
	if err != nil {
		return nil, err
	}

	vm = &BootcVMMac{
		socketFile: filepath.Join(params.User.CacheDir(), longId[:12]+"-console.sock"),
		BootcVMCommon: BootcVMCommon{
			imageID:       longId,
			cacheDir:      cacheDir,
			diskImagePath: filepath.Join(cacheDir, config.DiskImage),
			pidFile:       filepath.Join(cacheDir, config.RunPidFile),
			user:          params.User,
			cacheDirLock:  lock,
		},
	}

	return vm, nil

}

func (b *BootcVMMac) CloseConnection() {
	return //no-op when using qemu
}

func (b *BootcVMMac) PrintConsole() (err error) {
	return nil
}

func (b *BootcVMMac) GetConfig() (cfg *BootcVMConfig, err error) {
	cfg, err = b.LoadConfigFile()
	if err != nil {
		return
	}

	vmPidFile := filepath.Join(b.cacheDir, config.RunPidFile)
	pid, _ := utils.ReadPidFile(vmPidFile)
	if pid != -1 && utils.IsProcessAlive(pid) {
		cfg.Running = true
	} else {
		cfg.Running = false
	}

	return
}

func (b *BootcVMMac) Run(params RunVMParameters) (err error) {
	b.sshPort = params.SSHPort
	b.removeVm = params.RemoveVm
	b.background = params.Background
	b.cmd = params.Cmd
	b.hasCloudInit = params.CloudInitData
	b.cloudInitDir = params.CloudInitDir
	b.vmUsername = params.VMUser
	b.sshIdentity = params.SSHIdentity

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("following executable symlink: %w", err)
	}

	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("getting executable absolute path: %w", err)
	}

	args := []string{"vmmon", b.imageID, b.vmUsername, b.sshIdentity, strconv.Itoa(b.sshPort)}
	cmd := exec.Command(execPath, args...)

	logrus.Debugf("Executing: %v", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func (b *BootcVMMac) Delete() error {
	logrus.Debugf("Deleting Mac VM %s", b.cacheDir)

	isRunning, err := b.IsRunning()
	if err != nil {
		return fmt.Errorf("checking if VM is running: %w", err)
	}

	if !isRunning {
		return nil
	}

	pid, err := utils.ReadPidFile(b.pidFile)
	if err != nil {
		return fmt.Errorf("reading pid file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found while attempting to delete VM: %w", err)
	}

	return process.Signal(os.Interrupt)
}

func (b *BootcVMMac) IsRunning() (bool, error) {
	pidFileExists, err := utils.FileExists(b.pidFile)
	if !pidFileExists {
		logrus.Debugf("pid file does not exist, assuming VM is not running.")
		return false, nil //assume if pid is missing the VM is not running
	}

	pid, err := utils.ReadPidFile(b.pidFile)
	if err != nil {
		return false, fmt.Errorf("reading pid file: %w", err)
	}

	if pid != -1 && utils.IsProcessAlive(pid) {
		return true, nil
	} else {
		return false, nil
	}
}

func (b *BootcVMMac) Exists() (bool, error) {
	return utils.FileExists(b.pidFile)
}

func (v *BootcVMMac) Unlock() error {
	return v.cacheDirLock.Unlock()
}
