package cmd

import (
	"fmt"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/define"
	"github.com/containers/podman-bootc/pkg/user"
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
	usr, err := user.NewUser()
	if err != nil {
		return err
	}

	if removeAll {
		return pruneAll(usr)
	}

	id := args[0]
	fullImageId, err := usr.Storage().SearchByPrefix(id)
	if err != nil {
		return fmt.Errorf("searching for ID %s: %w", id, err)
	}
	if fullImageId == nil {
		return fmt.Errorf("local installation '%s' does not exists", id)
	}

	return prune(usr, *fullImageId)
}

func prune(usr user.User, id define.FullImageId) error {
	_, unlock, err := usr.Storage().GetExclusiveOrAdd(id)
	if err != nil {
		return fmt.Errorf("unable to lock the VM cache: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logrus.Errorf("unable to unlock VM %s: %v", id, err)
		}
	}()

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    string(id),
		LibvirtUri: config.LibvirtUri,
		User:       usr,
	})
	if err != nil {
		return fmt.Errorf("unable to get VM %s: %v", id, err)
	}

	defer func() {
		bootcVM.CloseConnection()
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

func pruneAll(usr user.User) error {
	ids, err := usr.Storage().List()
	if err != nil {
		return err
	}

	for _, id := range ids {
		err := prune(usr, id)
		if err != nil {
			logrus.Errorf("unable to remove %s: %v", id, err)
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
