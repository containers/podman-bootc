package cmd

import (
	"fmt"

	"podman-bootc/pkg/bootc"
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"
	"podman-bootc/pkg/vm"

	"github.com/spf13/cobra"
)

type osVmConfig struct {
	User            string
	CloudInitDir    string
	KsFile          string
	Background      bool
	NoCredentials   bool
	RemoveVm        bool // Kill the running VM when it exits
	RemoveDiskImage bool // After exit of the VM, remove the disk image
}

var (
	// listCmd represents the hello command
	runCmd = &cobra.Command{
		Use:          "run",
		Short:        "Run a bootc container as a VM",
		Long:         "Run a bootc container as a VM",
		Args:         cobra.MinimumNArgs(1),
		RunE:         doRun,
		SilenceUsage: true,
	}

	vmConfig = osVmConfig{}
)

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&vmConfig.User, "user", "u", "root", "--user <user name> (default: root)")

	runCmd.Flags().StringVar(&vmConfig.CloudInitDir, "cloudinit", "", "--cloudinit [[transport:]cloud-init data directory] (transport: cdrom | imds)")

	runCmd.Flags().BoolVar(&vmConfig.NoCredentials, "no-creds", false, "Do not inject default SSH key via credentials; also implies --background")
	runCmd.Flags().BoolVarP(&vmConfig.Background, "background", "B", false, "Do not spawn SSH, run in background")
	runCmd.Flags().BoolVar(&vmConfig.RemoveVm, "rm", false, "Remove the VM and it's disk when the SSH session exits. Cannot be used with --background")
}

func doRun(flags *cobra.Command, args []string) error {
	// create the disk image
	idOrName := args[0]
	bootcDisk := bootc.NewBootcDisk(idOrName)
	err := bootcDisk.Install()

	if err != nil {
		return fmt.Errorf("unable to install bootc image: %w", err)
	}

	//start the VM
	sshPort, err := utils.GetFreeLocalTcpPort()
	if err != nil {
		return fmt.Errorf("unable to get free port for SSH: %w", err)
	}

	sshIdentity := config.MachineSshKeyPriv
	background := vmConfig.Background
	if vmConfig.NoCredentials {
		sshIdentity = ""
		if !background {
			fmt.Print("No credentials provided for SSH, using --background by default")
			background = true
		}
	}

	cmd := args[1:]
	vmParameters := vm.BootcVMParameters{
		RemoveVm:      vmConfig.RemoveVm,
		Background:    background,
		Directory:     bootcDisk.GetDirectory(),
		User:          vmConfig.User,
		Name:          idOrName,
		Cmd:           cmd,
		ImageID:       bootcDisk.GetImageId(),
		ImageDigest:   bootcDisk.GetDigest(),
		CloudInitDir:  vmConfig.CloudInitDir,
		NoCredentials: vmConfig.NoCredentials,
		CloudInitData: flags.Flags().Changed("cloudinit"),
		SSHIdentity:   sshIdentity,
		SSHPort:       sshPort,
	}

	bootcVM, err := vm.NewVM(vmParameters)

	err = bootcVM.Run()
	if err != nil {
		return fmt.Errorf("runBootcVM: %w", err)
	}

	// write down the config file
	if err = bootcVM.WriteConfig(); err != nil {
		return err
	}

	if !vmConfig.Background {
		// wait for VM
		//time.Sleep(5 * time.Second) // just for now
		err = bootcVM.WaitForSSHToBeReady()
		if err != nil {
			return fmt.Errorf("WaitSshReady: %w", err)
		}

		// ssh into it
		err = bootcVM.RunSSH(cmd)
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}

		// Always remove when executing a command
		if vmConfig.RemoveVm || len(cmd) > 0 {
			err = bootcVM.ForceDelete() //delete the VM, but keep the disk image
			if err != nil {
				return fmt.Errorf("unable to remove VM from cache: %w", err)
			}
		}
	}

	return nil
}
