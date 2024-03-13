package cmd

import (
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/user"
	"podman-bootc/pkg/vm"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop ID",
	Short: "Stop an existing OS Container machine",
	Long:  "Stop an existing OS Container machine",
	Args:  cobra.ExactArgs(1),
	RunE:  doStop,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func doStop(_ *cobra.Command, args []string) (err error) {
	user, err := user.NewUser()
	if err != nil {
		return err
	}

	id := args[0]
	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    id,
		LibvirtUri: config.LibvirtUri,
		User:       user,
	})
	if err != nil {
		return err
	}
	return bootcVM.ForceDelete()
}
