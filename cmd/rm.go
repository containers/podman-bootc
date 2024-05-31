package cmd

import (
	"fmt"
	"os"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	force     = false
	removeAll = false
	rmCmd     = &cobra.Command{
		Use:   "rm <ID>",
		Short: "Remove installed bootc VMs",
		Long:  "Remove installed bootc VMs",
		Args:  oneOrAll(),
		RunE:  doRemove,
	}
)

func init() {
	RootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVar(&removeAll, "all", false, "Removes all non-running bootc VMs")
	rmCmd.Flags().BoolVarP(&force, "force", "f", false, "Terminate a running VM")
}

func oneOrAll() cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 1 && !removeAll {
			return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
		}
		if len(args) != 0 && removeAll {
			return fmt.Errorf("accepts 0 arg(s), received %d", len(args))
		}
		return nil
	}
}

func doRemove(_ *cobra.Command, args []string) error {
	if removeAll {
		return pruneAll()
	}

	return prune(args[0])
}

func prune(id string) error {
	user, err := user.NewUser()
	if err != nil {
		return err
	}

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    id,
		LibvirtUri: config.LibvirtUri,
		User:       user,
		Locking:    utils.Exclusive,
	})
	if err != nil {
		return fmt.Errorf("unable to get VM %s: %v", id, err)
	}

	// Let's be explicit instead of relying on the defer exec order
	defer func() {
		bootcVM.CloseConnection()
		if err := bootcVM.Unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", id, err)
		}
	}()

	if force {
		err := forceKillVM(bootcVM)
		if err != nil {
			return fmt.Errorf("unable to force kill %s", id)
		}
	} else {
		err := killVM(bootcVM)
		if err != nil {
			return fmt.Errorf("unable to kill %s", id)
		}
	}

	return nil
}

func pruneAll() error {
	user, err := user.NewUser()
	if err != nil {
		return err
	}

	files, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			vmID := f.Name()
			err := prune(vmID)
			if err != nil {
				logrus.Errorf("unable to remove %s: %v", vmID, err)
			}
		}
	}

	return nil
}

func killVM(bootcVM vm.BootcVM) (err error) {
	var isRunning bool
	isRunning, err = bootcVM.IsRunning()
	if err != nil {
		return fmt.Errorf("unable to check if VM is running: %v", err)
	}

	if isRunning {
		return fmt.Errorf("VM is currently running. Stop it first or use the -f flag.")
	} else {
		err = bootcVM.Delete()
		if err != nil {
			return
		}
	}

	return bootcVM.DeleteFromCache()
}

func forceKillVM(bootcVM vm.BootcVM) (err error) {
	err = bootcVM.Delete()
	if err != nil {
		return
	}

	return bootcVM.DeleteFromCache()
}
