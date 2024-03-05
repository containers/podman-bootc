package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"
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
	vmDir, err := config.BootcImagePath(id)
	if err != nil {
		return err
	}

	if !force {
		vmPidFile := filepath.Join(vmDir, config.RunPidFile)
		pid, _ := utils.ReadPidFile(vmPidFile)
		if pid != -1 && utils.IsProcessAlive(pid) {
			return fmt.Errorf("bootc container '%s' must be stopped first", id)
		}
	} else {
		err = forceKillVM(vmDir)
		if err != nil {
			logrus.Warningf("unable to kill %s", vmDir)
		}
	}

	return os.RemoveAll(vmDir)
}

func pruneAll() error {
	files, err := os.ReadDir(config.CacheDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			vmDir := filepath.Join(config.CacheDir, f.Name())
			vmPidFile := filepath.Join(vmDir, config.RunPidFile)
			if !force {
				pid, _ := utils.ReadPidFile(vmPidFile)
				if pid != -1 && !utils.IsProcessAlive(pid) {
					continue
				}
			} else {
				err = forceKillVM(vmDir)
				if err != nil {
					logrus.Warningf("unable to kill %s", vmDir)
				}
			}
			if err := os.RemoveAll(vmDir); err != nil {
				logrus.Warningf("unable to remove %s", vmDir)
			}
		}
	}

	return nil
}

func forceKillVM(vmDir string) (err error) {
	var bootcVM vm.BootcVM
	if runtime.GOOS == "darwin" {
		bootcVM = vm.NewBootcVMMacByDirectory(vmDir)
	} else {
		bootcVM, err = vm.NewBootcVMLinuxById(filepath.Base(vmDir))
		if err != nil {
			return err
		}
	}
	return bootcVM.Kill()
}
