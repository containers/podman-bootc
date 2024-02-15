package vmrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"podmanbootc/pkg/config"
	"podmanbootc/pkg/smbios"
)

func RunVM(vmDir string, sshPort int, user, sshIdentity string, ciData bool, ciPort int) error {
	var args []string
	args = append(args, "-accel", "kvm", "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", sshPort)
	args = append(args, "-nic", nicCmd)
	//args = append(args, "-nographic")

	vmPidFile := filepath.Join(vmDir, "run.pid")
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(vmDir, config.BootcDiskImage)
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
			ciDataIso := filepath.Join(vmDir, config.BootcCiDataIso)
			args = append(args, "-cdrom", ciDataIso)
		}
	}

	if sshIdentity != "" {
		smbiosCmd, err := smbios.OemString(user, sshIdentity)
		if err != nil {
			return err
		}

		args = append(args, "-smbios", smbiosCmd)
	}

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
