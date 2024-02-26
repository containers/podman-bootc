package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/disk"
	"podman-bootc/pkg/podman"
	"podman-bootc/pkg/ssh"
	"podman-bootc/pkg/utils"
	"podman-bootc/pkg/vm"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type osVmConfig struct {
	User            string
	CloudInitDir    string
	KsFile          string
	Background      bool
	RemoveVm        bool // Kill the running VM when it exits
	RemoveDiskImage bool // After exit of the VM, remove the disk image
}

var (
	// listCmd represents the hello command
	runCmd = &cobra.Command{
		Use:          "run",
		Short:        "Run a bootc container as a VM",
		Long:         "Run a bootc container as a VM",
		Args:         cobra.ExactArgs(1),
		RunE:         doBoot,
		SilenceUsage: true,
	}

	vmConfig = osVmConfig{}
)

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&vmConfig.User, "user", "u", "root", "--user <user name> (default: root)")

	runCmd.Flags().StringVar(&vmConfig.CloudInitDir, "cloudinit", "", "--cloudinit [[transport:]cloud-init data directory] (transport: cdrom | imds)")

	runCmd.Flags().BoolVarP(&vmConfig.Background, "background", "B", false, "Do not spawn SSH, run in background")
	runCmd.Flags().BoolVar(&vmConfig.RemoveVm, "rm", false, "Kill the running VM when it exits, requires --interactive")

}

func doBoot(flags *cobra.Command, args []string) error {
	idOrName := args[0]

	imageDigest, imageId, err := podman.GetOciImage(idOrName)
	if err != nil {
		return err
	}

	// Create VM cache dir; one per oci bootc image
	vmDir := filepath.Join(config.CacheDir, imageId)
	if err := os.MkdirAll(vmDir, os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %w", err)
	}

	// install
	start := time.Now()
	if err := disk.GetOrInstallImage(vmDir, idOrName, imageDigest); err != nil {
		return fmt.Errorf("installImage: %w", err)
	}
	elapsed := time.Since(start)
	logrus.Debugf("installImage elapsed: %v", elapsed)

	// run the new image
	privkey, _, err := podman.MachineSSHKey()
	if err != nil {
		return fmt.Errorf("getting podman ssh")
	}

	// cloud-init required?
	ciPort := -1 // for http transport
	ciData := flags.Flags().Changed("cloudinit")
	if ciData {
		ciPort, err = vm.SetCloudInit(imageDigest, vmConfig.CloudInitDir)
		if err != nil {
			return fmt.Errorf("setting up cloud init failed: %w", err)
		}
	}

	sshPort, err := utils.GetFreeLocalTcpPort()
	if err != nil {
		return fmt.Errorf("ssh getFreeTcpPort: %w", err)
	}

	err = vm.Run(vmDir, sshPort, vmConfig.User, privkey, ciData, ciPort)
	if err != nil {
		return fmt.Errorf("runBootcVM: %w", err)
	}

	// write down the config file
	if err := config.WriteConfig(vmDir, sshPort, privkey); err != nil {
		return err
	}

	if !vmConfig.Background {
		// wait for VM
		//time.Sleep(5 * time.Second) // just for now
		err = vm.WaitSshReady(vmDir, sshPort)
		if err != nil {
			return fmt.Errorf("WaitSshReady: %w", err)
		}

		// ssh into it
		cmd := args[1:]
		err = ssh.CommonSSH(vmConfig.User, privkey, idOrName, sshPort, cmd)
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}

		// Always remove when executing a command
		if vmConfig.RemoveVm || len(cmd) > 0 {
			// stop the new VM
			//poweroff := []string{"poweroff"}
			//err = CommonSSH("root", DefaultIdentity, name, sshPort, poweroff)
			err = vm.Kill(vmDir)
			if err != nil {
				return fmt.Errorf("poweroff: %w", err)
			}
		}
	}

	return nil
}
