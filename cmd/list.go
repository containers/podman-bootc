package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/user"
	"podman-bootc/pkg/utils"

	"github.com/spf13/cobra"
)

// listCmd represents the hello command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed OS Containers",
	Long:  "List installed OS Containers",
	RunE:  doList,
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func doList(_ *cobra.Command, _ []string) error {
	vmList, err := collectVmInfo()
	if err != nil {
		return err
	}

	fmt.Printf("%-30s \t\t %15s\n", "ID", "VM PID")
	for name, pid := range vmList {
		fmt.Printf("%-30s \t\t %10s\n", name, pid)
	}
	return nil
}

func collectVmInfo() (map[string]string, error) {
	user, err := user.NewUser()
	if err != nil {
		return nil, err
	}

	vmList := make(map[string]string)

	files, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			vmPidFile := filepath.Join(user.CacheDir(), f.Name(), config.RunPidFile)
			pid, _ := utils.ReadPidFile(vmPidFile)
			pidRep := "-"
			if pid != -1 && utils.IsProcessAlive(pid) {
				pidRep = strconv.Itoa(pid)
			}
			vmList[f.Name()[:12]] = pidRep
		}
	}
	return vmList, nil
}
