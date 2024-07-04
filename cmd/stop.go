package cmd

import (
	"fmt"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/vm"

	"github.com/sirupsen/logrus"
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
	usr, err := user.NewUser()
	if err != nil {
		return err
	}

	id := args[0]
	fullImageId, err := usr.Storage().SearchByPrefix(id)
	if err != nil {
		return fmt.Errorf("searching for ID %s: %w", id, err)
	}
	if fullImageId == nil {
		return fmt.Errorf("local installation '%s' does not exists", id)
	}

	_, unlock, err := usr.Storage().GetExclusiveOrAdd(*fullImageId)
	if err != nil {
		return fmt.Errorf("unable to lock the VM cache: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logrus.Errorf("unable to unlock VM %s: %v", id, err)
		}
	}()

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    string(*fullImageId),
		LibvirtUri: config.LibvirtUri,
		User:       usr,
	})
	if err != nil {
		return err
	}

	defer func() {
		bootcVM.CloseConnection()
	}()

	return bootcVM.Delete()
}
