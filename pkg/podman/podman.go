package podman

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"
	ocispec "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types"
)

type RunVMContainerOptions struct {
	ContainerStoragePath string
	ConfigDir            string
	OutputDir            string
	SocketDir            string
	LibvirtSocketDir     string
}

func detectLocalPodman() string {
	return ""
}

type VMContainer struct {
	contID     string
	image      string
	socketPath string
	opts       *RunVMContainerOptions
}

func ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	execCreateOptions := &handlers.ExecCreateConfig{
		ExecConfig: types.ExecConfig{
			Tty:          true,
			AttachStdin:  true,
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          cmd,
		},
	}
	execID, err := containers.ExecCreate(ctx, containerID, execCreateOptions)
	if err != nil {
		return "", fmt.Errorf("exec create failed: %w", err)
	}
	// Prepare streams
	var stdoutBuf, stderrBuf bytes.Buffer
	var stdout io.Writer = &stdoutBuf
	var stderr io.Writer = &stderrBuf
	// Start exec and attach
	err = containers.ExecStartAndAttach(ctx, execID, &containers.ExecStartAndAttachOptions{
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		AttachOutput: utils.Ptr(true),
		AttachError:  utils.Ptr(true),
	})
	if err != nil {
		return "", fmt.Errorf("exec start failed: %w", err)
	}

	// Handle output and errors
	if stderrBuf.Len() > 0 {
		return "", fmt.Errorf("stderr: %s", stderrBuf.String())
	}

	return stdoutBuf.String(), nil
}

func (c *VMContainer) GetBootArtifacts() (string, string, error) {
	ctx, err := connectPodman(c.socketPath)
	if err != nil {
		return "", "", fmt.Errorf("Failed to connect to Podman service: %v", err)
	}
	isRunning, err := isContainerRunning(ctx, c.contID)
	if err != nil {
		return "", "", err
	}
	if !isRunning {
		return "", "", fmt.Errorf("the VM container isn't running")
	}
	findKernel := []string{"find", "/bootc-data/usr/lib/modules/", "-name", "vmlinuz", "-type", "f"}
	findInitrd := []string{"find", "/bootc-data/usr/lib/modules/", "-name", "initramfs.img", "-type", "f"}
	out, err := ExecInContainer(ctx, c.contID, findKernel)
	if err != nil {
		return "", "", err
	}
	kernel := strings.Trim(out, "\r\n")
	out, err = ExecInContainer(ctx, c.contID, findInitrd)
	if err != nil {
		return "", "", err
	}
	initrd := strings.Trim(out, "\r\n")

	return kernel, initrd, nil
}

func NewVMContainer(image, socketPath string, opts *RunVMContainerOptions) *VMContainer {
	return &VMContainer{
		image:      image,
		socketPath: socketPath,
		opts:       opts,
	}
}

func (c *VMContainer) Stop() error {
	ctx, err := connectPodman(c.socketPath)
	if err != nil {
		return fmt.Errorf("Failed to connect to Podman service: %v", err)
	}
	if err := containers.Stop(ctx, c.contID, &containers.StopOptions{}); err != nil {
		return fmt.Errorf("failed to stop the bootc container: %v", err)
	}
	if _, err := containers.Remove(ctx, c.contID, &containers.RemoveOptions{}); err != nil {
		return fmt.Errorf("failed to stop the bootc container: %v", err)
	}

	return nil
}

func (c *VMContainer) Run() error {
	ctx, err := connectPodman(c.socketPath)
	if err != nil {
		return fmt.Errorf("Failed to connect to Podman service: %v", err)
	}

	c.contID, err = createVMContainer(ctx, c.image, c.opts)
	if err != nil {
		return err
	}

	if err := containers.Start(ctx, c.contID, &containers.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start the bootc container: %v", err)
	}

	isRunning, err := isContainerRunning(ctx, c.contID)
	if err != nil {
		return err
	}
	if !isRunning {
		return fmt.Errorf("the VM container %s isn't running", c.contID)
	}
	return err
}

func isContainerRunning(ctx context.Context, name string) (bool, error) {
	inspectData, err := containers.Inspect(ctx, name, nil)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Check if it's running
	return inspectData.State.Running, nil
}

func pullImage(ctx context.Context, image string) error {
	if _, err := images.Pull(ctx, image, &images.PullOptions{}); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}

	return nil
}

func createVMContainer(ctx context.Context, image string, opts *RunVMContainerOptions) (string, error) {
	if err := pullImage(ctx, image); err != nil {
		return "", err
	}
	specGen := &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Command: []string{"/entrypoint.sh"},
			Stdin:   utils.Ptr(true),
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: vm.VMImage,
			ImageVolumes: []*specgen.ImageVolume{
				{
					Destination: vm.BootcDir,
					Source:      image,
					ReadWrite:   true,
				},
			},
			Devices: []ocispec.LinuxDevice{
				{
					Path: "/dev/kvm",
					Type: "char",
				},
				{
					Path: "/dev/vhost-net",
					Type: "char",
				},
				{
					Path: "/dev/vhost-vsock",
					Type: "char",
				},
				{
					Path: "/dev/vhost-vsock",
					Type: "char",
				},
			},
			Mounts: []ocispec.Mount{
				{
					Destination: vm.ContainerStoragePath,
					Source:      opts.ContainerStoragePath,
					Type:        "bind",
				},
				{
					Destination: vm.OutputDir,
					Source:      opts.OutputDir,
					Type:        "bind",
				},
				{
					Destination: vm.ConfigDir,
					Source:      opts.ConfigDir,
					Type:        "bind",
				},
				{
					Destination: vm.SocketDir,
					Source:      opts.SocketDir,
					Type:        "bind",
				},
				{
					Destination: vm.LibvirtSocketDir,
					Source:      opts.LibvirtSocketDir,
					Type:        "bind",
				},
			},
		},
		ContainerSecurityConfig: specgen.ContainerSecurityConfig{
			Privileged:  utils.Ptr(true),
			SelinuxOpts: []string{"type:unconfined_t"},
		},
		ContainerCgroupConfig: specgen.ContainerCgroupConfig{},
		ContainerNetworkConfig: specgen.ContainerNetworkConfig{
			PublishExposedPorts: utils.Ptr(true),
			Expose:              map[uint16]string{uint16(vm.VNCPort): "tcp"},
		},
	}
	if err := specGen.Validate(); err != nil {
		return "", err
	}
	response, err := containers.CreateWithSpec(ctx, specGen, &containers.CreateOptions{})
	if err != nil {
		return "", err
	}

	log.Debugf("Run VM container ID: %s", response.ID)

	return response.ID, nil
}

func connectPodman(socketPath string) (context.Context, error) {
	const (
		retryInterval = 5 * time.Second
		timeout       = 5 * time.Minute
	)

	deadline := time.Now().Add(timeout)

	var ctx context.Context
	var err error

	for time.Now().Before(deadline) {
		ctx, err = bindings.NewConnection(context.Background(), fmt.Sprintf("unix:%s", socketPath))
		if err == nil {
			log.Debugf("Connected to Podman successfully!")
			return ctx, nil
		}

		log.Debugf("Failed to connect to Podman. Retrying in %s seconds...", retryInterval.String())
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("Unable to connect to Podman after %v: %v", timeout, err)
}

func createBootcContainer(ctx context.Context, image string, bootcCmdLine []string) (string, error) {
	log.Debugf("Create bootc container with cmdline: %v", bootcCmdLine)
	specGen := &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Command: bootcCmdLine,
			Stdin:   utils.Ptr(true),
			PidNS: specgen.Namespace{
				NSMode: specgen.Host,
			},
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: image,
			Mounts: []ocispec.Mount{
				{
					Destination: "/var/lib/containers",
					Source:      "/var/lib/containers",
					Type:        "bind",
				},
				{
					Destination: "/var/lib/containers/storage",
					Source:      vm.ContainerStoragePath,
					Type:        "bind",
				},
				{
					Destination: "/dev",
					Source:      "/dev",
					Type:        "bind",
				},
				{
					Destination: "/output",
					Source:      vm.OutputDir,
					Type:        "bind",
				},
				{
					Destination: "/config",
					Source:      vm.ConfigDir,
					Type:        "bind",
				},
			},
		},
		ContainerSecurityConfig: specgen.ContainerSecurityConfig{
			Privileged:  utils.Ptr(true),
			SelinuxOpts: []string{"type:unconfined_t"},
		},
		ContainerCgroupConfig: specgen.ContainerCgroupConfig{},
	}
	if err := specGen.Validate(); err != nil {
		return "", err
	}
	response, err := containers.CreateWithSpec(ctx, specGen, &containers.CreateOptions{})
	if err != nil {
		return "", err
	}

	return response.ID, nil
}

func fetchLogsAfterExit(ctx context.Context, containerID string) error {
	stdoutCh := make(chan string)
	stderrCh := make(chan string)

	// Start log streaming
	go func() {
		logOpts := new(containers.LogOptions).WithFollow(true).WithStdout(true).WithStderr(true)

		err := containers.Logs(ctx, containerID, logOpts, stdoutCh, stderrCh)
		if err != nil {
			log.Errorf("Error streaming logs: %v\n", err)
		}
		close(stdoutCh)
		close(stderrCh)
	}()

	go func() {
		for line := range stdoutCh {
			fmt.Fprintf(os.Stdout, "%s", line)
		}
	}()
	go func() {
		for line := range stderrCh {
			fmt.Fprintf(os.Stderr, "%s", line)
		}
	}()

	exitCode, err := containers.Wait(ctx, containerID, new(containers.WaitOptions).
		WithCondition([]define.ContainerStatus{define.ContainerStateExited}))
	if err != nil {
		return fmt.Errorf("failed to wait for container: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("bootc command failed: %d", exitCode)
	}

	return nil
}

func RunPodmanCmd(socketPath string, image string, bootcCmdLine []string) error {
	ctx, err := connectPodman(socketPath)
	if err != nil {
		return fmt.Errorf("Failed to connect to Podman service: %v", err)
	}

	name, err := createBootcContainer(ctx, image, bootcCmdLine)
	if err != nil {
		return fmt.Errorf("failed to create the bootc container: %v", err)
	}

	if err := containers.Start(ctx, name, &containers.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start the bootc container: %v", err)
	}

	if err := fetchLogsAfterExit(ctx, name); err != nil {
		return fmt.Errorf("failed executing bootc: %v", err)
	}

	return nil
}

func DefaultPodmanSocket() string {
	if envSock := os.Getenv("DOCKER_HOST"); envSock != "" {
		return envSock
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" {
		return filepath.Join(runtimeDir, "podman", "podman.sock")
	}
	usr, err := user.Current()
	if err == nil && usr.Uid != "0" {
		return "/run/user/" + usr.Uid + "/podman/podman.sock"
	}

	return "/run/podman/podman.sock"
}

func DefaultContainerStorage() string {
	usr, err := user.Current()
	if err == nil && usr.Uid != "0" {
		homeDir := os.Getenv("HOME")
		if homeDir != "" {
			return filepath.Join(homeDir, ".local/share/containers/storage")
		}
	}

	return "/var/lib/containers/storage"
}
