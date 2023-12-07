package cmd

import (
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:     "stop NAME",
	Short:   "Stop an existing OS Container machine",
	Long:    "Stop an existing OS Container machine",
	Args:    cobra.ExactArgs(1),
	Example: `osc stop fedora-base`,
	RunE:    stopVm,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func stopVm(_ *cobra.Command, args []string) error {
	name := args[0]
	runCfg, err := LoadRunningVmFromDisk(name)
	if err != nil {
		return err
	}

	vm := NewVMPartial(name)
	poweroff := []string{"poweroff"}
	return CommonSSH("root", vm.SshPriKey, name, int(runCfg.SshPort), poweroff)
}
