package cmd

import (
	"gitlab.com/bootc-org/podman-bootc/pkg/config"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
	"gitlab.com/bootc-org/podman-bootc/pkg/utils"
	"gitlab.com/bootc-org/podman-bootc/pkg/vm"

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
	user, err := user.NewUser()
	if err != nil {
		return err
	}

	id := args[0]

	vm, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    id,
		User:       user,
		LibvirtUri: config.LibvirtUri,
		Locking:    utils.Shared,
	})

	if err != nil {
		return err
	}

	// Let's be explicit instead of relying on the defer exec order
	defer func() {
		vm.CloseConnection()
		if err := vm.Unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", id, err)
		}
	}()

	err = vm.SetUser(sshUser)
	if err != nil {
		return err
	}

	cmd := make([]string, 0)
	if len(args) > 1 {
		cmd = args[1:]
	}

	ExitCode, err = utils.WithExitCode(vm.RunSSH(cmd))
	return err
}
