package cmd

import (
	"context"
	"fmt"
	"os"
	filepath "path/filepath"

	"github.com/containers/podman-bootc/pkg/podman"
	"github.com/containers/podman-bootc/pkg/vm"
	"github.com/containers/podman-bootc/pkg/vm/domain"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/spf13/cobra"
	log "github.com/sirupsen/logrus"
)

type installCmd struct {
	image            string
	bootcCmdLine     []string
	artifactsDir     string
	diskPath         string
	ctx              context.Context
	socket           string
	podmanSocketDir  string
	libvirtDir       string
	outputImage      string
	containerStorage string
	configPath       string
	outputPath       string
	installVM        *vm.InstallVM
}

func filterCmdlineArgs(args []string) ([]string, error) {
	sepIndex := -1
	for i, arg := range args {
		if arg == "--" {
			sepIndex = i
			break
		}
	}
	if sepIndex == -1 {
		return nil, fmt.Errorf("no command line specified")
	}

	return args[sepIndex+1:], nil
}

func NewInstallCommand() *cobra.Command {
	c := installCmd{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the OS Containers",
		Long:  "Run bootc install to build the OS Containers. Specify the bootc cmdline after the '--'",
		RunE:  c.doInstall,
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = ""
	}
	cacheDir = filepath.Join(cacheDir, "bootc")
	cmd.PersistentFlags().StringVar(&c.image, "bootc-image", "", "bootc-vm container image")
	cmd.PersistentFlags().StringVar(&c.artifactsDir, "dir", cacheDir, "directory where the artifacts are extracted")
	cmd.PersistentFlags().StringVar(&c.outputPath, "output-dir", "", "directory to store the output results")
	cmd.PersistentFlags().StringVar(&c.outputImage, "output-image", "", "path of the image to use for the installation")
	cmd.PersistentFlags().StringVar(&c.configPath, "config-dir", "", "path where to find the config.toml")
	cmd.PersistentFlags().StringVar(&c.containerStorage, "container-storage", podman.DefaultContainerStorage(), "Container storage to use")
	cmd.PersistentFlags().StringVar(&c.socket, "podman-socket", podman.DefaultPodmanSocket(), "path to the podman socket")
	if args, err := filterCmdlineArgs(os.Args); err == nil {
		c.bootcCmdLine = args
	}

	return cmd
}

func init() {
	RootCmd.AddCommand(NewInstallCommand())
}

func (c *installCmd) validateArgs() error {
	if c.image == "" {
		return fmt.Errorf("the bootc-image cannot be empty")
	}
	if c.artifactsDir == "" {
		return fmt.Errorf("the artifacts directory path cannot be empty")
	}
	if c.outputImage == "" {
		return fmt.Errorf("the output-image needs to be set")
	}
	if c.outputPath == "" {
		return fmt.Errorf("the output-path needs to be set")
	}
	if c.configPath == "" {
		return fmt.Errorf("the config-dir needs to be set")
	}
	if c.containerStorage == "" {
		return fmt.Errorf("the container storage cannot be empty")
	}
	if c.socket == "" {
		return fmt.Errorf("the socket for podman cannot be empty")
	}
	if len(c.bootcCmdLine) == 0 {
		return fmt.Errorf("the bootc commandline needs to be specified after the '--'")
	}
	var err error
	c.ctx, err = bindings.NewConnection(context.Background(), "unix://"+c.socket)
	if err != nil {
		return fmt.Errorf("failed to connect to podman at %s: %v", c.socket, err)
	}

	return nil
}

func (c *installCmd) installBuildVM(kernel, initrd string) error {
	image := filepath.Join(c.outputPath, c.outputImage)
	outputImageFormat, err := domain.GetDiskInfo(image)
	if err != nil {
		return err
	}
	c.installVM = vm.NewInstallVM(filepath.Join(c.libvirtDir, "virtqemud-sock"), vm.InstallOptions{
		OutputFormat: outputImageFormat,
		OutputImage:  filepath.Join(vm.OutputDir, c.outputImage), // Path relative to the container filesystem
		Root:         false,
		Kernel:       kernel,
		Initrd:       initrd,
	})
	if err := c.installVM.Run(); err != nil {
		return err
	}

	return nil
}

func (c *installCmd) doInstall(_ *cobra.Command, _ []string) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	c.libvirtDir = filepath.Join(c.artifactsDir, "libvirt")
	if _, err := os.Stat(c.libvirtDir); os.IsNotExist(err) {
		if err := os.Mkdir(c.libvirtDir, 0755); err != nil {
			return err
		}
	}
	c.podmanSocketDir = filepath.Join(c.artifactsDir, "podman")
	if _, err := os.Stat(c.podmanSocketDir); os.IsNotExist(err) {
		if err := os.Mkdir(c.podmanSocketDir, 0755); err != nil {
			return err
		}
	}
	remoteSocket := filepath.Join(c.podmanSocketDir, "podman-vm.sock")
	vmCont := podman.NewVMContainer(c.image, c.socket, &podman.RunVMContainerOptions{
		ContainerStoragePath: c.containerStorage,
		ConfigDir:            c.configPath,
		OutputDir:            c.outputPath,
		SocketDir:            c.podmanSocketDir,
		LibvirtSocketDir:     c.libvirtDir,
	})
	if err := vmCont.Run(); err != nil {
		return err
	}
	defer vmCont.Stop()

	kernel, initrd, err := vmCont.GetBootArtifacts()
	if err != nil {
		return err
	}
	log.Debugf("Boot artifacts kernel: %s and initrd: %s", kernel, initrd)

	if err := c.installBuildVM(kernel, initrd); err != nil {
		return err
	}
	defer c.installVM.Stop()

	if err := podman.RunPodmanCmd(remoteSocket, c.image, c.bootcCmdLine); err != nil {
		return err
	}

	return nil
}
