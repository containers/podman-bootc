package cmd

import (
	"fmt"
	"os"
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/vm"
	"runtime"

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
	bootcVM, err := getVM(id)
	if err != nil {
		return fmt.Errorf("unable to get VM %s: %v", id, err)
	}

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
	files, err := os.ReadDir(config.CacheDir)
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

func getVM(id string) (bootcVM vm.BootcVM, err error) {
	if runtime.GOOS == "darwin" {
		bootcVM, err = vm.NewBootcVMMacById(id)
		if err != nil {
			return
		}
	} else {
		bootcVM, err = vm.NewBootcVMLinuxById(id)
		if err != nil {
			return
		}
	}
	return
}

func killVM(bootcVM vm.BootcVM) (err error) {
	isRunning, err := bootcVM.IsRunning()
	if err != nil {
		return
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
	err = bootcVM.ForceDelete()
	if err != nil {
		return
	}

	return bootcVM.DeleteFromCache()
}
