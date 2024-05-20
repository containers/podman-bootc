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
	id := args[0]

	user, err := user.NewUser()
	if err != nil {
		return err
	}

	//take an exclusive lock on the cache directory
	fullImageId, err := utils.FullImageIdFromPartial(id, user)
	if err != nil {
		return err
	}
	cacheDir, err := cache.NewCache(fullImageId, user)
	if err != nil {
		return err
	}
	err = cacheDir.Lock(cache.Exclusive)
	if err != nil {
		return err
	}

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    id,
		LibvirtUri: config.LibvirtUri,
		User:       user,
	})
	if err != nil {
		return err
	}

	defer func() {
		// Let's be explicit instead of relying on the defer exec order
		bootcVM.CloseConnection()
		if err := cacheDir.Unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", id, err)
		}
	}()

	return bootcVM.Delete()
}
