package cmd

import (
	"fmt"
	"os"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/define"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/vm"

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
	ids, err := user.Storage().List()
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		cfg, err := getVMInfo(user, libvirtUri, id)
		if err != nil {
			logrus.Warningf("skipping vm %s reason: %v", id, err)
			continue
		}

		vmList = append(vmList, *cfg)
	}
	return vmList, nil
}

func getVMInfo(user user.User, libvirtUri string, imageId define.FullImageId) (*vm.BootcVMConfig, error) {
	guard, unlock, err := user.Storage().Get(imageId)
	if err != nil {
		return nil, fmt.Errorf("unable to lock the VM cache: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logrus.Warningf("unable to unlock VM %s: %v", imageId, err)
		}
	}()

	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:    string(imageId),
		User:       user,
		LibvirtUri: libvirtUri,
	})

	if err != nil {
		return nil, err
	}

	defer func() {
		bootcVM.CloseConnection()
	}()

	cfg, err := bootcVM.GetConfig(guard)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
