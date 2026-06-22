package bib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const defaultBibImage = "quay.io/centos-bootc/bootc-image-builder"

type BuildOption struct {
	BibContainerImage string
	Config            string
	Output            string
	Filesystem        string
	Format            string
	Arch              string
	BibExtraArgs      []string
}

func Build(ctx context.Context, user user.User, imageNameOrId string, quiet bool, buildOption BuildOption) error {
	outputInfo, err := os.Stat(buildOption.Output)
	if err != nil {
		return fmt.Errorf("output directory %s: %w", buildOption.Output, err)
	}

	if !outputInfo.IsDir() {
		return fmt.Errorf("%s is not a directory ", buildOption.Output)
	}

	_, err = os.Stat(buildOption.Config)
	if err != nil {
		return fmt.Errorf("config file %s: %w", buildOption.Config, err)
	}

	// Let's convert both the config file and the output directory to their absolute paths.
	buildOption.Output, err = filepath.Abs(buildOption.Output)
	if err != nil {
		return fmt.Errorf("getting output directory absolute path: %w", err)
	}

	buildOption.Config, err = filepath.Abs(buildOption.Config)
	if err != nil {
		return fmt.Errorf("getting config file absolute path: %w", err)
	}

	// We assume the user's home directory is accessible from the podman machine VM, this
	// will fail if any of the output or the config file are outside the user's home directory.
	if !strings.HasPrefix(buildOption.Output, user.HomeDir()) {
		return errors.New("the output directory must be inside the user's home directory")
	}

	if !strings.HasPrefix(buildOption.Config, user.HomeDir()) {
		return errors.New("the output directory must be inside the user's home directory")
	}

	// Let's pull the bootc image container if necessary
	imageInspect, err := utils.PullAndInspect(ctx, imageNameOrId)
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	imageFullName := imageInspect.RepoTags[0]

	if buildOption.BibContainerImage == "" {
		label, found := imageInspect.Labels["bootc.diskimage-builder"]
		if found && label != "" {
			buildOption.BibContainerImage = label
		} else {
			buildOption.BibContainerImage = defaultBibImage
		}
	}

	// Let's pull the Bootc Image Builder if necessary
	_, err = utils.PullAndInspect(ctx, buildOption.BibContainerImage)
	if err != nil {
		return fmt.Errorf("pulling bootc image builder image: %w", err)
	}

	// BIB doesn't work with just the image ID or short name, it requires the image full name
	bibContainer, err := createBibContainer(ctx, buildOption.BibContainerImage, imageFullName, buildOption)
	if err != nil {
		return fmt.Errorf("failed to create image builder container: %w", err)
	}

	err = containers.Start(ctx, bibContainer.ID, &containers.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start image builder container: %w", err)
	}

	// Ensure we've cancelled the container attachment when exiting this function, as
	// it takes over stdout/stderr handling
	attachCancelCtx, cancelAttach := context.WithCancel(ctx)
	defer cancelAttach()

	if !quiet {
		attachOpts := new(containers.AttachOptions).WithStream(true)
		if err := containers.Attach(attachCancelCtx, bibContainer.ID, os.Stdin, os.Stdout, os.Stderr, nil, attachOpts); err != nil {
			return fmt.Errorf("attaching image builder container: %w", err)
		}
	}

	exitCode, err := containers.Wait(ctx, bibContainer.ID, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for image builder container: %w", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to run image builder")
	}

	return nil
}

func createBibContainer(ctx context.Context, bibContainerImage, imageFullName string, buildOption BuildOption) (types.ContainerCreateResponse, error) {
	privileged := true
	autoRemove := true
	labelNested := true
	terminal := true // Allocate pty so we can show progress bars, spinners etc.

	bibArgs := bibArguments(imageFullName, buildOption)

	s := &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Remove:       &autoRemove,
			Annotations:  map[string]string{"io.podman.annotations.label": "type:unconfined_t"},
			Terminal:     &terminal,
			Command:      bibArgs,
			SdNotifyMode: "container", // required otherwise crun will fail to open the sd-bus
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: bibContainerImage,
			Mounts: []specs.Mount{
				{
					Source:      buildOption.Config,
					Destination: "/config.toml",
					Type:        "bind",
				},
				{
					Source:      buildOption.Output,
					Destination: "/output",
					Type:        "bind",
					Options:     []string{"nosuid", "nodev"},
				},
				{
					Source:      "/var/lib/containers/storage",
					Destination: "/var/lib/containers/storage",
					Type:        "bind",
				},
			},
		},
		ContainerSecurityConfig: specgen.ContainerSecurityConfig{
			Privileged:  &privileged,
			LabelNested: &labelNested,
			SelinuxOpts: []string{"type:unconfined_t"},
		},
		ContainerNetworkConfig: specgen.ContainerNetworkConfig{
			NetNS: specgen.Namespace{
				NSMode: specgen.Bridge,
			},
		},
	}

	logrus.Debugf("Installing %s using %s", imageFullName, bibContainerImage)
	createResponse, err := containers.CreateWithSpec(ctx, s, &containers.CreateOptions{})
	if err != nil {
		return createResponse, fmt.Errorf("failed to create image builder container: %w", err)
	}
	return createResponse, nil
}

func bibArguments(imageNameOrId string, buildOption BuildOption) []string {
	args := []string{
		"--local", // we pull the image if necessary, so don't pull it from a registry
	}

	if buildOption.Filesystem != "" {
		args = append(args, "--rootfs", buildOption.Filesystem)
	}

	if buildOption.Arch != "" {
		args = append(args, "--target-arch", buildOption.Arch)
	}

	if buildOption.Format != "" {
		args = append(args, "--type", buildOption.Format)
	}

	args = append(args, buildOption.BibExtraArgs...)
	args = append(args, imageNameOrId)

	logrus.Debugf("BIB arguments: %v", args)
	return args
}
