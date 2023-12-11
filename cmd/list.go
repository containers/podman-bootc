package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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

	fmt.Printf("%-30s \t\t %5s \t\t %12s \t\t %15s\n", "NAME", "VCPUs", "MEMORY", "DISK SIZE")
	for _, vm := range vmList {
		fmt.Printf("%-30s \t\t %4d \t\t %8d MiB \t\t %11d GiB \t %10s\n",
			vm.Name, vm.Vcpu, vm.Mem, vm.DiskSize, vm.Status())
	}
	return nil
}

func collectVmInfo() (map[string]*VmConfig, error) {
	vmList := make(map[string]*VmConfig)

	files, err := os.ReadDir(ConfigDir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			vm, err := LoadVmFromDisk(f.Name())
			if err != nil {
				return nil, err
			}
			vmList[vm.Name] = vm
		}
	}
	return vmList, nil
}
