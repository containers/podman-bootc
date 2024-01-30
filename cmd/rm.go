package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
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
	vmDir, err := bootcImagePath(id)
	if err != nil {
		return err
	}

	vmPidFile := filepath.Join(vmDir, runPidFile)
	pid, _ := readPidFile(vmPidFile)
	if pid != -1 && isPidAlive(pid) {
		return fmt.Errorf("bootc container '%s' must be stopped first", id)
	}

	return os.RemoveAll(vmDir)
}
