package vm

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gitlab.com/bootc-org/podman-bootc/pkg/config"
	"gitlab.com/bootc-org/podman-bootc/pkg/utils"

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
	//qemu seems to asynchronously create the socket file
	//so this will wait up to a few seconds for socket to be created
	socketCreationTimeout := 5 * time.Second
	elapsed := 0 * time.Millisecond
	interval := 100 * time.Millisecond
	for elapsed < socketCreationTimeout {
		time.Sleep(interval) //always sleep a little bit at the start
		elapsed += interval
		if _, err = os.Stat(b.socketFile); err == nil {
			break
		}
	}

	c, err := net.Dial("unix", b.socketFile)
	if err != nil {
		return fmt.Errorf("error connecting to socket %s", err)
	}
	for {
		buf := make([]byte, 8192)
		_, err := c.Read(buf)
		if err != nil {
			return fmt.Errorf("error reading socket %s", err)
		}
		print(string(buf))
	}
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
	b.interactive = params.Interactive
	b.cmd = params.Cmd
	b.hasCloudInit = params.CloudInitData
	b.cloudInitDir = params.CloudInitDir
	b.vmUsername = params.VMUser
	b.sshIdentity = params.SSHIdentity

	if params.NoCredentials {
		b.sshIdentity = ""
		if b.interactive {
			fmt.Print("No credentials provided for SSH, running the VM in the background")
			b.interactive = false
		}
	}

	var args []string
	args = append(args, "-display", "none")
	args = append(args, "-chardev", fmt.Sprintf("socket,id=char0,server=on,wait=off,path=%s", b.socketFile), "-serial", "chardev:char0")

	args = append(args, "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	args = append(args, "-snapshot")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", b.sshPort)
	args = append(args, "-nic", nicCmd)

	vmPidFile := filepath.Join(b.cacheDir, "run.pid")
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(b.cacheDir, config.DiskImage)
	driveCmd := fmt.Sprintf("if=virtio,format=raw,file=%s", vmDiskImage)
	args = append(args, "-drive", driveCmd)

	err = b.ParseCloudInit()
	if err != nil {
		return err
	}

	if b.hasCloudInit {
		args = append(args, "-cdrom", b.cloudInitArgs)
	}

	if b.sshIdentity != "" {
		smbiosCmd, err := b.oemString()
		if err != nil {
			return err
		}

		args = append(args, "-smbios", smbiosCmd)
	}

	cmd, err := b.createQemuCommand()
	if err != nil {
		return err
	}

	cmd.Args = append(cmd.Args, args...)
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

func (b *BootcVMMac) createQemuCommand() (*exec.Cmd, error) {
	qemuInstallPath, err := getQemuInstallPath()
	if err != nil {
		return nil, err
	}

	path := qemuInstallPath + "/bin/qemu-system-aarch64"
	args := []string{
		"-accel", "hvf",
		"-cpu", "host",
		"-M", "virt,highmem=on",
		"-drive", "file=" + qemuInstallPath + "/share/qemu/edk2-aarch64-code.fd" + ",if=pflash,format=raw,readonly=on",
	}
	return exec.Command(path, args...), nil
}

// Search for a qemu binary, let's check if is shipped with podman v4
// or if it's installed using homebrew.
// This function will no longer be necessary as soon as we use libvirt on macos.
func getQemuInstallPath() (string, error) {
	dirs := []string{"/opt/homebrew", "/opt/podman/qemu"}
	for _, d := range dirs {
		qemuBinary := filepath.Join(d, "bin/qemu-system-aarch64")
		if _, err := os.Stat(qemuBinary); err == nil {
			return d, nil
		}
	}

	return "", errors.New("QEMU binary not found")
}

func (v *BootcVMMac) Unlock() error {
	return v.cacheDirLock.Unlock()
}
