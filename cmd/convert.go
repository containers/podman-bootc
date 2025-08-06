package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman-bootc/pkg/bib"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	convertCmd = &cobra.Command{
		Use:   "convert <image>",
		Short: "Creates a disk image using bootc-image-builder",
		Long:  "Creates a disk image using bootc-image-builder",
		Args:  cobra.ExactArgs(1),
		RunE:  doConvert,
	}
	options bib.BuildOption
	quiet   bool
)

func init() {
	RootCmd.AddCommand(convertCmd)
	convertCmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress output from disk image creation")
	convertCmd.Flags().StringVar(&options.Config, "config", "", "Image builder config file")
	convertCmd.Flags().StringVar(&options.Output, "output", ".", "output directory (default \".\")")
	// Corresponds to bib '--rootfs', we don't use 'rootfs' so to not be confused with podman's 'rootfs' options
	// Note: we cannot provide a default value for the filesystem, since this options will overwrite the one defined in
	// the image
	convertCmd.Flags().StringVar(&options.Filesystem, "filesystem", "", "Overrides the root filesystem (e.g. xfs, btrfs, ext4)")
	// Corresponds to bib '--type', using '--format' to be consistent with podman
	convertCmd.Flags().StringVar(&options.Format, "format", "qcow2", "Disk image type (ami, anaconda-iso, iso, qcow2, raw, vmdk) [default: qcow2]")
	// Corresponds to bib '--target-arch', using '--arch' to be consistent with podman
	convertCmd.Flags().StringVar(&options.Arch, "arch", "", "Build for the given target architecture (experimental)")

	options.BibContainerImage = os.Getenv("PODMAN_BOOTC_BIB_IMAGE")
	options.BibExtraArgs = strings.Fields(os.Getenv("PODMAN_BOOTC_BIB_EXTRA"))
}

func doConvert(_ *cobra.Command, args []string) (err error) {
	//get user info who is running the podman bootc command
	usr, err := user.NewUser()
	if err != nil {
		return fmt.Errorf("unable to get user: %w", err)
	}

	machine, err := utils.GetMachineContext()
	if err != nil {
		println(utils.PodmanMachineErrorMessage)
		logrus.Errorf("failed to connect to podman machine. Is podman machine running?\n%s", err)
		return err
	}

	idOrName := args[0]
	err = bib.Build(machine.Ctx, usr, idOrName, quiet, options)
	if err != nil {
		return err
	}

	return nil
}
