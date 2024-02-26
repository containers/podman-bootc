package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm NAME",
	Short:   "Remove installed OS Containers",
	Long:    "Remove installed OS Containers",
	Args:    cobra.ExactArgs(1),
	Example: `podman bootc rm 6c6c2fc015fe`,
	Run:     removeVmCmd,
}

func init() {
	RootCmd.AddCommand(rmCmd)
}

func removeVmCmd(_ *cobra.Command, args []string) {
	err := Remove(args[0])
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func Remove(id string) error {
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
