package vm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"

	streamarch "github.com/coreos/stream-metadata-go/arch"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

func createQemuCommand() *exec.Cmd {
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

func Run(vmDir string, sshPort int, user, sshIdentity string, ciData bool, ciPort int) error {
	var args []string
	args = append(args, "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	args = append(args, "-snapshot")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", sshPort)
	args = append(args, "-nic", nicCmd)
	//args = append(args, "-nographic")

	vmPidFile := filepath.Join(vmDir, "run.pid")
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(vmDir, config.DiskImage)
	driveCmd := fmt.Sprintf("if=virtio,format=raw,file=%s", vmDiskImage)
	args = append(args, "-drive", driveCmd)
	if ciData {
		if ciPort != -1 {
			// http cloud init data transport
			// FIXME: this IP address is qemu specific, it should be configurable.
			smbiosCmd := fmt.Sprintf("type=1,serial=ds=nocloud;s=http://10.0.2.2:%d/", ciPort)
			args = append(args, "-smbios", smbiosCmd)
		} else {
			// cdrom cloud init data transport
			ciDataIso := filepath.Join(vmDir, config.CiDataIso)
			args = append(args, "-cdrom", ciDataIso)
		}
	}

	if sshIdentity != "" {
		smbiosCmd, err := OemString(user, sshIdentity)
		if err != nil {
			return err
		}

		args = append(args, "-smbios", smbiosCmd)
	}

	cmd := createQemuCommand()
	cmd.Args = append(cmd.Args, args...)
	logrus.Debugf("Executing: %v", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func WaitSshReady(vmDir string, port int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	err = watcher.Add(vmDir)
	if err != nil {
		return fmt.Errorf("add watcher: %w", err)
	}

	vmPidFile := filepath.Join(vmDir, config.RunPidFile)
	for {
		exists, err := utils.FileExists(vmPidFile)
		if err != nil {
			return fmt.Errorf("check for file: %w", err)
		}

		if exists {
			break
		}

		select {
		case <-watcher.Events:
		case err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("unknown error")
			}
			return fmt.Errorf("watcher event: %w", err)
		}
	}

	for {
		if utils.IsPortOpen(port) {
			return nil
		}
	}
}

func Kill(vmDir string) error {
	vmPidFile := filepath.Join(vmDir, config.RunPidFile)
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
