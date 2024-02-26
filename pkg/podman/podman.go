package podman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"podman-bootc/pkg/config"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var ctx context.Context = func() (ctx context.Context) {
	if _, err := os.Stat(config.MachineSocket); err != nil {
		logrus.Errorf("podman machine socket is missing. Is podman machine running?\n%s", err)
		os.Exit(1)
		return
	}

	ctx, err := bindings.NewConnectionWithIdentity(
		context.Background(),
		fmt.Sprintf("unix://%s", config.MachineSocket),
		config.MachineSshKeyPriv,
		true)
	if err != nil {
		logrus.Errorf("failed to connect to the podman socket. Is podman machine running?\n%s", err)
		os.Exit(1)
		return
	}

	return ctx
}()

// PullImage fetches the image if not present
func PullImage(image string) (id string, digest string, err error) {
	pullPolicy := "missing"
	ids, err := images.Pull(ctx, image, &images.PullOptions{Policy: &pullPolicy})
	if err != nil {
		return "", "", fmt.Errorf("failed to pull image: %w", err)
	}

	if len(ids) == 0 {
		return "", "", fmt.Errorf("no ids returned from image pull")
	}

	if len(ids) > 1 {
		return "", "", fmt.Errorf("multiple ids returned from image pull")
	}

	inspectReport, err := images.GetImage(ctx, image, &images.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get image: %w", err)
	}

	return ids[0], inspectReport.Digest.String(), nil
}

// BootcInstallToDisk runs the bootc installer in a container to create a disk image
func BootcInstallToDisk(image string, disk *os.File) (err error) {
	createResponse, err := createContainer(image, disk)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// run the container to create the disk
	err = containers.Start(ctx, createResponse.ID, &containers.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return
}

func createContainer(image string, disk *os.File) (createResponse types.ContainerCreateResponse, err error) {
	envHost := true
	privileged := true

	s := &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Name: "podman-bootc-installer",
			Command: []string{
				"bootc", "install", "to-disk", "--via-loopback", "--generic-image",
				"--skip-fetch-check", "/output/" + filepath.Base(disk.Name()),
			},
			EnvHost: &envHost,
			PidNS:   specgen.Namespace{NSMode: specgen.Host},
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: image,
			Mounts: []specs.Mount{
				{
					Source:      "/var/lib/containers",
					Destination: "/var/lib/containers",
					Type:        "bind",
				},
				{
					Source:      "/dev",
					Destination: "/dev",
					Type:        "bind",
				},
			},
		},
		ContainerSecurityConfig: specgen.ContainerSecurityConfig{
			Privileged:  &privileged,
			SelinuxOpts: []string{"type:unconfined_t"}, // TODO: verify this in the container
		},
		ContainerNetworkConfig: specgen.ContainerNetworkConfig{
			NetNS: specgen.Namespace{
				NSMode: specgen.Host,
			},
		},
	}

	createResponse, err = containers.CreateWithSpec(ctx, s, &containers.CreateOptions{})
	if err != nil {
		return createResponse, fmt.Errorf("failed to create container: %w", err)
	}

	return
}
