package cmd

import (
	"os"

	"gitlab.com/bootc-org/podman-bootc/pkg/cache"
	"gitlab.com/bootc-org/podman-bootc/pkg/config"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
	"gitlab.com/bootc-org/podman-bootc/pkg/vm"

	"github.com/containers/common/pkg/report"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// listCmd represents the hello command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed OS Containers",
	Long:  "List installed OS Containers",
	RunE:  doList,
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func doList(_ *cobra.Command, _ []string) error {
	hdrs := report.Headers(vm.BootcVMConfig{}, map[string]string{
		"RepoTag":  "Repo",
		"DiskSize": "Size",
	})

	rpt := report.New(os.Stdout, "list")
	defer rpt.Flush()

	rpt, err := rpt.Parse(
		report.OriginPodman,
		"{{range . }}{{.Id}}\t{{.RepoTag}}\t{{.DiskSize}}\t{{.Created}}\t{{.Running}}\t{{.SshPort}}\n{{end -}}")

	if err != nil {
		return err
	}

	if err := rpt.Execute(hdrs); err != nil {
		return err
	}

	user, err := user.NewUser()
	if err != nil {
		return err
	}

	vmList, err := CollectVmList(user, config.LibvirtUri)
	if err != nil {
		return err
	}

	return rpt.Execute(vmList)
}

func CollectVmList(user user.User, libvirtUri string) (vmList []vm.BootcVMConfig, err error) {
	files, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			cfg, err := getVMInfo(user, libvirtUri, f.Name())
			if err != nil {
				logrus.Warningf("skipping vm %s reason: %v", f.Name(), err)
				continue
			}

			vmList = append(vmList, *cfg)
		}
	}
	return vmList, nil
}

func getVMInfo(user user.User, libvirtUri string, fullImageId string) (*vm.BootcVMConfig, error) {
	cacheDir, err := cache.NewCache(fullImageId, user)
	if err != nil {
		return nil, err
	}
	err = cacheDir.Lock(cache.Shared)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := cacheDir.Unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", fullImageId, err)
		}
	}()

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    fullImageId,
		User:       user,
		LibvirtUri: libvirtUri,
	})

	if err != nil {
		return nil, err
	}

	defer func() {
		bootcVM.CloseConnection()
	}()

	cfg, err := bootcVM.GetConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
