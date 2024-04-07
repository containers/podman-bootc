package bootc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"podman-bootc/pkg/config"

	"podman-bootc/pkg/user"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const diskSize = 10 * 1024 * 1024 * 1024
const imageMetaXattr = "user.bootc.meta"

// diskFromContainerMeta is serialized to JSON in a user xattr on a disk image
type diskFromContainerMeta struct {
	// imageDigest is the digested sha256 of the container that was used to build this disk
	ImageDigest string `json:"imageDigest"`
}

type BootcDisk struct {
	ImageNameOrId           string
	User                    user.User
	Ctx                     context.Context
	ImageId                 string
	RepoTag                 string
	CreatedAt               time.Time
	Directory               string
	file                    *os.File
	bootcInstallContainerId string
}

// create singleton for easy cleanup
var (
	instance     *BootcDisk
	instanceOnce sync.Once
)

func NewBootcDisk(imageNameOrId string, ctx context.Context, user user.User) *BootcDisk {
	instanceOnce.Do(func() {
		instance = &BootcDisk{
			ImageNameOrId: imageNameOrId,
			Ctx:           ctx,
			User:          user,
		}
	})
	return instance
}

func (p *BootcDisk) GetDirectory() string {
	return p.Directory
}

func (p *BootcDisk) GetImageId() string {
	return p.ImageId
}

// GetSize returns the size of the disk in bytes
func (p *BootcDisk) GetSize() int {
	return diskSize
}

// GetRepoTag returns the repository of the container image
func (p *BootcDisk) GetRepoTag() string {
	return p.RepoTag
}

// GetCreatedAt returns the creation time of the disk image
func (p *BootcDisk) GetCreatedAt() time.Time {
	return p.CreatedAt
}

func (p *BootcDisk) Install(quiet bool) (err error) {
	p.CreatedAt = time.Now()

	err = p.pullImage()
	if err != nil {
		return
	}

	err = p.getOrInstallImageToDisk(quiet)
	if err != nil {
		return
	}

	elapsed := time.Since(p.CreatedAt)
	logrus.Debugf("installImage elapsed: %v", elapsed)

	return
}

func (p *BootcDisk) Cleanup() (err error) {
	force := true
	if p.bootcInstallContainerId != "" {
		_, err := containers.Remove(p.Ctx, p.bootcInstallContainerId, &containers.RemoveOptions{Force: &force})
		if err != nil {
			return fmt.Errorf("failed to remove bootc install container: %w", err)
		}
	}

	return
}

// getOrInstallImageToDisk checks if the disk is present and if not, installs the image to a new disk
func (p *BootcDisk) getOrInstallImageToDisk(quiet bool) error {
	diskPath := filepath.Join(p.Directory, config.DiskImage)
	f, err := os.Open(diskPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		logrus.Debugf("No existing disk image found")
		return p.bootcInstallImageToDisk(quiet)
	}
	logrus.Debug("Found existing disk image, comparing digest")
	defer f.Close()
	buf := make([]byte, 4096)
	len, err := unix.Fgetxattr(int(f.Fd()), imageMetaXattr, buf)
	if err != nil {
		// If there's no xattr, just remove it
		os.Remove(diskPath)
		logrus.Debugf("No %s xattr found", imageMetaXattr)
		return p.bootcInstallImageToDisk(quiet)
	}
	bufTrimmed := buf[:len]
	var serializedMeta diskFromContainerMeta
	if err := json.Unmarshal(bufTrimmed, &serializedMeta); err != nil {
		logrus.Warnf("failed to parse serialized meta from %s (%v) %v", diskPath, buf, err)
		return p.bootcInstallImageToDisk(quiet)
	}

	logrus.Debugf("previous disk digest: %s current digest: %s", serializedMeta.ImageDigest, p.ImageId)
	if serializedMeta.ImageDigest == p.ImageId {
		return nil
	}

	return p.bootcInstallImageToDisk(quiet)
}

// bootcInstallImageToDisk creates a disk image from a bootc container
func (p *BootcDisk) bootcInstallImageToDisk(quiet bool) (err error) {
	println("Creating bootc disk image...")
	p.file, err = os.CreateTemp(p.Directory, "podman-bootc-tempdisk")
	if err != nil {
		return err
	}
	if err := syscall.Ftruncate(int(p.file.Fd()), diskSize); err != nil {
		return err
	}
	logrus.Debugf("Created %s with size %v", p.file.Name(), diskSize)
	doCleanupDisk := true
	defer func() {
		if doCleanupDisk {
			os.Remove(p.file.Name())
		}
	}()

	err = p.runInstallContainer(quiet)
	if err != nil {
		return fmt.Errorf("failed to create disk image: %w", err)
	}
	serializedMeta := diskFromContainerMeta{
		ImageDigest: p.ImageId,
	}
	buf, err := json.Marshal(serializedMeta)
	if err != nil {
		return err
	}
	if err := unix.Fsetxattr(int(p.file.Fd()), imageMetaXattr, buf, 0); err != nil {
		return fmt.Errorf("failed to set xattr: %w", err)
	}
	diskPath := filepath.Join(p.Directory, config.DiskImage)

	if err := os.Rename(p.file.Name(), diskPath); err != nil {
		return fmt.Errorf("failed to rename to %s: %w", diskPath, err)
	}
	doCleanupDisk = false

	return nil
}

// pullImage fetches the container image if not present
func (p *BootcDisk) pullImage() (err error) {
	pullPolicy := "missing"
	ids, err := images.Pull(p.Ctx, p.ImageNameOrId, &images.PullOptions{Policy: &pullPolicy})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	if len(ids) == 0 {
		return fmt.Errorf("no ids returned from image pull")
	}

	if len(ids) > 1 {
		return fmt.Errorf("multiple ids returned from image pull")
	}

	image, err := images.GetImage(p.Ctx, p.ImageNameOrId, &images.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	imageId := ids[0]
	p.ImageId = imageId
	p.RepoTag = image.RepoTags[0]

	// Create VM cache dir; one per oci bootc image
	p.Directory = filepath.Join(p.User.CacheDir(), imageId)
	if err := os.MkdirAll(p.Directory, os.ModePerm); err != nil {
		return fmt.Errorf("error while making bootc disk directory: %w", err)
	}

	return
}

// runInstallContainer runs the bootc installer in a container to create a disk image
func (p *BootcDisk) runInstallContainer(quiet bool) (err error) {
	createResponse, err := p.createInstallContainer()
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	p.bootcInstallContainerId = createResponse.ID //save the id for possible cleanup
	logrus.Debugf("Created install container, id=%s", createResponse.ID)

	// run the container to create the disk
	err = containers.Start(p.Ctx, p.bootcInstallContainerId, &containers.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	logrus.Debugf("Started install container")

	var exitCode int32
	if quiet {
		//wait for the container to finish
		logrus.Debugf("Waiting for container completion")
		exitCode, err = containers.Wait(p.Ctx, p.bootcInstallContainerId, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container: %w", err)
		}
	} else {
		// stream logs to stdout and stderr
		stdOut := make(chan string)
		stdErr := make(chan string)
		logErrors := make(chan error)

		var wg sync.WaitGroup
		go func() {
			follow := true
			defer close(stdOut)
			defer close(stdErr)
			trueV := true
			err = containers.Logs(p.Ctx, p.bootcInstallContainerId, &containers.LogOptions{Follow: &follow, Stdout: &trueV, Stderr: &trueV}, stdOut, stdErr)
			if err != nil {
				logErrors <- err
			}

			close(logErrors)
		}()

		wg.Add(1)
		go func() {
			for str := range stdOut {
				fmt.Print(str)
			}
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			for str := range stdErr {
				fmt.Fprintf(os.Stderr, "%s", str)
			}
			wg.Done()
		}()

		//wait for the container to finish
		logrus.Debugf("Waiting for container completion (streaming output)")
		exitCode, err = containers.Wait(p.Ctx, p.bootcInstallContainerId, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container: %w", err)
		}

		if err := <-logErrors; err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		// Ensure the streams are done
		wg.Wait()
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to run bootc install")
	}

	return
}

// createInstallContainer creates a container to run the bootc installer
func (p *BootcDisk) createInstallContainer() (createResponse types.ContainerCreateResponse, err error) {
	privileged := true
	autoRemove := true
	labelNested := true

	targetEnv := make(map[string]string)
	if v, ok := os.LookupEnv("BOOTC_INSTALL_LOG"); ok {
		targetEnv["RUST_LOG"] = v
	}

	s := &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Command: []string{
				"bootc", "install", "to-disk", "--via-loopback", "--generic-image",
				"--skip-fetch-check", "/output/" + filepath.Base(p.file.Name()),
			},
			PidNS:       specgen.Namespace{NSMode: specgen.Host},
			Remove:      &autoRemove,
			Annotations: map[string]string{"io.podman.annotations.label": "type:unconfined_t"},
			Env:         targetEnv,
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: p.ImageNameOrId,
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
				{
					Source:      p.Directory,
					Destination: "/output",
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

	createResponse, err = containers.CreateWithSpec(p.Ctx, s, &containers.CreateOptions{})
	if err != nil {
		return createResponse, fmt.Errorf("failed to create container: %w", err)
	}

	return
}
