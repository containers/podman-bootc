package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strconv"
)

// listCmd represents the hello command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed OS Containers",
	Long:  "List installed OS Containers",
	Run:   list,
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func list(_ *cobra.Command, _ []string) {
	err := doList()
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doList() error {
	vmList, err := collectVmInfo()
	if err != nil {
		return err
	}

	fmt.Printf("%-30s \t\t %15s\n", "IMAGE ID", "VM PID")
	for name, pid := range vmList {
		fmt.Printf("%-30s \t\t %10s\n", name, pid)
	}
	return nil
}

func collectVmInfo() (map[string]string, error) {
	vmList := make(map[string]string)

	files, err := os.ReadDir(CacheDir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() && f.Name() != "machine" && f.Name() != "netinst" {
			vmPidFile := filepath.Join(CacheDir, f.Name(), runPidFile)
			pid, _ := readPidFile(vmPidFile)
			pidRep := "-"
			if pid != -1 && isPidAlive(pid) {
				pidRep = strconv.Itoa(pid)
			}
			vmList[f.Name()[:12]] = pidRep
		}
	}
	return vmList, nil
}
