package cmd

import (
	"fmt"
	"time"

	"github.com/containers/podman-bootc/pkg/bootc"
	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/credentials"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"

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
	Quiet           bool
	TLSVerify       bool
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

	vmConfig                = osVmConfig{}
	diskImageConfigInstance = bootc.DiskImageConfig{}
)

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&vmConfig.User, "user", "u", "root", "--user <user name> (default: root)")

	runCmd.Flags().StringVar(&vmConfig.CloudInitDir, "cloudinit", "", "--cloudinit <cloud-init data directory>")

	runCmd.Flags().StringVar(&diskImageConfigInstance.Filesystem, "filesystem", "", "Override the root filesystem (e.g. xfs, btrfs, ext4)")
	runCmd.Flags().BoolVarP(&vmConfig.Background, "background", "B", false, "Do not spawn SSH, run in background")
	runCmd.Flags().BoolVar(&vmConfig.RemoveVm, "rm", false, "Remove the VM and it's disk when the SSH session exits. Cannot be used with --background")
	runCmd.Flags().BoolVar(&vmConfig.Quiet, "quiet", false, "Suppress output from bootc disk creation and VM boot console")
	runCmd.Flags().StringVar(&diskImageConfigInstance.RootSizeMax, "root-size-max", "", "Maximum size of root filesystem in bytes; optionally accepts M, G, T suffixes")
	runCmd.Flags().StringVar(&diskImageConfigInstance.DiskSize, "disk-size", "", "Allocate a disk image of this size in bytes; optionally accepts M, G, T suffixes")
	runCmd.Flags().BoolVar(&vmConfig.TLSVerify, "tls-verify", true, "Require HTTPS and verify certificates when accessing the registry")
}

func doRun(flags *cobra.Command, args []string) error {
	//get user info who is running the podman bootc command
	user, err := user.NewUser()
	if err != nil {
		return fmt.Errorf("unable to get user: %w", err)
	}

	machine, err := utils.GetMachineContext()
	if err != nil {
		println(utils.PodmanMachineErrorMessage)
		logrus.Errorf("failed to connect to podman machine. Is podman machine running?\n%s", err)
		return err
	}

	// create the disk image
	idOrName := args[0]
	bootcDisk := bootc.NewBootcDisk(idOrName, machine.Ctx, user)

	// skip tls verification if tls-verfiy flag is set to false
	bootcDisk.SkipTLSVerify = !vmConfig.TLSVerify

	err = bootcDisk.Install(vmConfig.Quiet, diskImageConfigInstance)

	if err != nil {
		return fmt.Errorf("unable to install bootc image: %w", err)
	}

	//start the VM
	println("Booting the VM...")
	sshPort, err := utils.GetFreeLocalTcpPort()
	if err != nil {
		return fmt.Errorf("unable to get free port for SSH: %w", err)
	}

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    bootcDisk.GetImageId(),
		User:       user,
		LibvirtUri: config.LibvirtUri,
		Locking:    utils.Shared,
	})

	if err != nil {
		return fmt.Errorf("unable to initialize VM: %w", err)
	}

	// Let's be explicit instead of relying on the defer exec order
	defer func() {
		bootcVM.CloseConnection()
		if err := bootcVM.Unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", bootcDisk.GetImageId(), err)
		}
	}()

	sSHIdentityPath, err := credentials.Generatekeys(bootcVM.CacheDir())
	if err != nil {
		return fmt.Errorf("unable to generate ssh key: %w", err)
	}

	cmd := args[1:]
	err = bootcVM.Run(vm.RunVMParameters{
		Cmd:           cmd,
		CloudInitDir:  vmConfig.CloudInitDir,
		CloudInitData: flags.Flags().Changed("cloudinit"),
		RemoveVm:      vmConfig.RemoveVm,
		Background:    vmConfig.Background,
		SSHPort:       sshPort,
		SSHIdentity:   sSHIdentityPath,
		VMUser:        vmConfig.User,
	})

	if err != nil {
		return fmt.Errorf("runBootcVM: %w", err)
	}

	// write down the config file
	if err = bootcVM.WriteConfig(*bootcDisk); err != nil {
		return err
	}

	if !vmConfig.Background {
		if !vmConfig.Quiet {
			go func() {
				err := bootcVM.PrintConsole()
				if err != nil {
					logrus.Errorf("error printing VM console: %v", err)
				}
			}()

			err = bootcVM.WaitForSSHToBeReady()
			if err != nil {
				return fmt.Errorf("WaitSshReady: %w", err)
			}

			// the PrintConsole routine is suddenly stopped without waiting for
			// the print buffer to be flushed, this can lead to the consoel output
			// printing after the ssh prompt begins. Sleeping for a second
			// should prevent this from happening on most systems.
			//
			// The libvirt console stream API blocks while waiting for data, so
			// cleanly stopping the routing via a channel is not possible.
			time.Sleep(1 * time.Second)
		} else {
			err = bootcVM.WaitForSSHToBeReady()
			if err != nil {
				return fmt.Errorf("WaitSshReady: %w", err)
			}
		}

		// ssh into the VM
		ExitCode, err = utils.WithExitCode(bootcVM.RunSSH(cmd))
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}

		// Always remove when executing a command
		if vmConfig.RemoveVm || len(cmd) > 0 {
			err = bootcVM.Delete() //delete the VM, but keep the disk image
			if err != nil {
				return fmt.Errorf("unable to remove VM from cache: %w", err)
			}
		}
	}

	return nil
}
