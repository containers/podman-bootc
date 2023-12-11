package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:     "stop NAME",
	Short:   "Stop an existing OS Container machine",
	Long:    "Stop an existing OS Container machine",
	Args:    cobra.ExactArgs(1),
	Example: `osc stop fedora-base`,
	Run:     stopVm,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func stopVm(_ *cobra.Command, args []string) {
	err := doStopVm(args[0])
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doStopVm(name string) error {

	runCfg, err := LoadRunningVmFromDisk(name)
	if err != nil {
		return err
	}

	vm := NewVMPartial(name)
	poweroff := []string{"poweroff"}
	return CommonSSH("root", vm.SshPriKey, name, int(runCfg.SshPort), poweroff)
}
