package cmd

import (
	"gitlab.com/bootc-org/podman-bootc/pkg/cache"
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

	//take a read only lock on the cache directory
	fullImageId, err := utils.FullImageIdFromPartial(id, user)
	if err != nil {
		return err
	}
	cacheDir, err := cache.NewCache(fullImageId, user)
	if err != nil {
		return err
	}
	err = cacheDir.Lock(cache.Shared)
	if err != nil {
		return err
	}

	vm, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    id,
		User:       user,
		LibvirtUri: config.LibvirtUri,
	})

	if err != nil {
		return err
	}

	defer func() {
		// Let's be explicit instead of relying on the defer exec order
		vm.CloseConnection()
		if err := cacheDir.Unlock(); err != nil {
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
