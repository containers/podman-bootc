package cmd

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"

	"github.com/spf13/cobra"
)

var (
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

	vmPidFile := filepath.Join(vmDir, config.RunPidFile)
	pid, _ := utils.ReadPidFile(vmPidFile)
	if pid != -1 && utils.IsProcessAlive(pid) {
		return fmt.Errorf("bootc container '%s' must be stopped first", id)
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
			pid, _ := utils.ReadPidFile(vmPidFile)
			if pid == -1 || !utils.IsProcessAlive(pid) {
				if err := os.RemoveAll(vmDir); err != nil {
					logrus.Warningf("unable to remove %s", vmDir)
				}
			}
		}
	}

	return nil
}
