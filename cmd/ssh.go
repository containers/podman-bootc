package cmd

import (
	"fmt"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <ID>",
	Short: "SSH into an existing OS Container machine",
	Long:  "SSH into an existing OS Container machine",
	Args:  cobra.MinimumNArgs(1),
	RunE:  doSsh,
}
var sshUser string

func init() {
	RootCmd.AddCommand(sshCmd)
	sshCmd.Flags().StringVarP(&sshUser, "user", "u", "root", "--user <user name> (default: root)")
}

func doSsh(_ *cobra.Command, args []string) error {
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

	guard, unlock, err := usr.Storage().Get(*fullImageId)
	if err != nil {
		return fmt.Errorf("unable to lock the VM cache: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", id, err)
		}
	}()

	vm, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    string(*fullImageId),
		User:       usr,
		LibvirtUri: config.LibvirtUri,
	})

	if err != nil {
		return err
	}
	defer func() {
		vm.CloseConnection()
	}()

	err = vm.SetUser(sshUser)
	if err != nil {
		return err
	}

	cmd := make([]string, 0)
	if len(args) > 1 {
		cmd = args[1:]
	}

	ExitCode, err = utils.WithExitCode(vm.RunSSH(guard, cmd))
	return err
}
