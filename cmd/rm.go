package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm NAME",
	Short:   "Remove installed OS Containers",
	Long:    "Remove installed OS Containers",
	Args:    cobra.ExactArgs(1),
	Example: `osc rm fedora-base`,
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

func Remove(name string) error {
	exists, err := isCreated(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("VM '%s' does not exists", name)
	}
	vm := NewVMPartial(name)
	if vm.Status() != Stopped {
		return fmt.Errorf("VM '%s' must be stopped first", name)
	}

	key1, key2 := vm.SshKeys()
	_ = os.Remove(key1)
	_ = os.Remove(key2)
	os.RemoveAll(vm.RunDir())
	return os.RemoveAll(vm.ConfigDir())
}
