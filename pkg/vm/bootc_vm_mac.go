package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"

	streamarch "github.com/coreos/stream-metadata-go/arch"
	"github.com/sirupsen/logrus"
)

type BootcVMMac struct {
	BootcVMCommon
	ciPort int
	ciData bool
}

func NewBootcVMMacByDirectory(directory string) (vm BootcVMMac) {
	return BootcVMMac{
		BootcVMCommon: BootcVMCommon{
			directory: directory,
		},
	}
}

func NewBootcVMMac(params BootcVMParameters) (BootcVMMac, error) {
	return BootcVMMac{
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
			imageDigest:   params.ImageDigest,
			ciData:        params.CloudInitData,
		},
	}, nil
}

func (b BootcVMMac) Run() error {
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
	if b.ciData {
		if b.ciPort != -1 {
			// http cloud init data transport
			// FIXME: this IP address is qemu specific, it should be configurable.
			smbiosCmd := fmt.Sprintf("type=1,serial=ds=nocloud;s=http://10.0.2.2:%d/", b.ciPort)
			args = append(args, "-smbios", smbiosCmd)
		} else {
			// cdrom cloud init data transport
			ciDataIso := filepath.Join(b.directory, config.CiDataIso)
			args = append(args, "-cdrom", ciDataIso)
		}
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

func (b BootcVMMac) Kill() error {
	vmPidFile := filepath.Join(b.directory, config.RunPidFile)
	pid, err := utils.ReadPidFile(vmPidFile)
	if err != nil {
		return fmt.Errorf("reading pid file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	return process.Signal(os.Interrupt)
}

func (b BootcVMMac) createQemuCommand() *exec.Cmd {
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
