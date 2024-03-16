package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/ssh"
	"podman-bootc/pkg/utils"

	streamarch "github.com/coreos/stream-metadata-go/arch"
	"github.com/sirupsen/logrus"
)

type BootcVMMac struct {
	BootcVMCommon
}

func NewVMById(id string) (vm *BootcVMMac, err error) {
	directory, err := config.BootcImagePath(id)
	if err != nil {
		return
	}

	return &BootcVMMac{
		BootcVMCommon: BootcVMCommon{
			directory: directory,
		},
	}, nil
}

func NewVM(params BootcVMParameters) (*BootcVMMac, error) {
	return &BootcVMMac{
		BootcVMCommon: BootcVMCommon{
			user:          params.User,
			directory:     params.Directory,
			diskImagePath: filepath.Join(params.Directory, config.DiskImage),
			sshIdentity:   params.SSHIdentity,
			sshPort:       params.SSHPort,
			removeVm:      params.RemoveVm,
			background:    params.Background,
			name:          params.Name,
			cmd:           params.Cmd,
			pidFile:       filepath.Join(params.Directory, config.RunPidFile),
			imageID:       params.ImageID,
			hasCloudInit:  params.CloudInitData,
			cloudInitDir:  params.CloudInitDir,
		},
	}, nil
}

func (b *BootcVMMac) Run() error {
	var args []string
	args = append(args, "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	args = append(args, "-snapshot")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", b.sshPort)
	args = append(args, "-nic", nicCmd)
	//args = append(args, "-nographic")

	vmPidFile := filepath.Join(b.directory, "run.pid")
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(b.directory, config.DiskImage)
	driveCmd := fmt.Sprintf("if=virtio,format=raw,file=%s", vmDiskImage)
	args = append(args, "-drive", driveCmd)

	err := b.ParseCloudInit()
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

	cmd := b.createQemuCommand()
	cmd.Args = append(cmd.Args, args...)
	logrus.Debugf("Executing: %v", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func (b *BootcVMMac) Delete() error {
	logrus.Debugf("Deleting Mac VM %s", b.directory)

	vmPidFile := filepath.Join(b.directory, config.RunPidFile)
	pid, err := utils.ReadPidFile(vmPidFile)
	if err != nil {
		return fmt.Errorf("reading pid file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found while attempting to delete VM: %w", err)
	}

	return process.Signal(os.Interrupt)
}

func (b *BootcVMMac) Shutdown() error {
	isRunning, err := b.IsRunning()
	if err != nil {
		return fmt.Errorf("unable to determine if VM is running: %w", err)
	}

	if isRunning {
		id := b.imageID
		cfg, err := config.LoadConfig(id)
		if err != nil {
			return err
		}

		poweroff := []string{"poweroff"}
		return ssh.CommonSSH(b.user, cfg.SshIdentity, id, cfg.SshPort, poweroff)
	} else {
		logrus.Infof("Unable to shutdown VM. It is not not running.")
		return nil
	}
}

func (b *BootcVMMac) ForceDelete() error {
	err := b.Shutdown()
	if err != nil {
		return fmt.Errorf("unable to shutdown VM: %w", err)
	}

	err = b.Delete()
	if err != nil {
		return fmt.Errorf("unable to delete VM: %w", err)
	}

	return nil
}

func (b *BootcVMMac) IsRunning() (bool, error) {
	pidFileExists, err := utils.FileExists(b.pidFile)
	if !pidFileExists {
		logrus.Debugf("pid file does not exist, assuming VM is not running.")
		return false, nil //assume if pid is missing the VM is not running
	}

	vmPidFile := filepath.Join(b.directory, config.RunPidFile)
	pid, err := utils.ReadPidFile(vmPidFile)
	if err != nil {
		return false, fmt.Errorf("reading pid file: %w", err)
	}

	if pid != -1 && utils.IsProcessAlive(pid) {
		return true, nil
	} else {
		return false, nil
	}
}

func (v *BootcVMMac) Exists() (bool, error) {
	return utils.FileExists(v.pidFile)
}

func (b *BootcVMMac) createQemuCommand() *exec.Cmd {
	var path string
	args := []string{}
	podmanqemuPath := "/opt/podman/qemu"
	if runtime.GOOS == "darwin" {
		path = podmanqemuPath + "/bin/qemu-system-aarch64"
		args = append(args,
			"-accel", "hvf",
			"-cpu", "host",
			"-M", "virt,highmem=on",
			"-drive", "file="+podmanqemuPath+"/share/qemu/edk2-aarch64-code.fd"+",if=pflash,format=raw,readonly=on",
		)
	} else {
		arch := streamarch.CurrentRpmArch()
		path = "qemu-system-" + arch
		args = append(args, "-accel", "kvm")
	}
	return exec.Command(path, args...)
}
