package bootc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// As a baseline heuristic we double the size of
// the input container to support in-place updates.
// This is planned to be more configurable in the
// future.  See also bootc-image-builder
const containerSizeToDiskSizeMultiplier = 2
const diskSizeMinimum = 10 * 1024 * 1024 * 1024 // 10GB
const imageMetaXattr = "user.bootc.meta"

// DiskImageConfig defines configuration for the
type DiskImageConfig struct {
	Filesystem  string
	RootSizeMax string
	DiskSize    string
}

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
	imageData               *types.ImageInspectReport
	RepoTag                 string
	CreatedAt               time.Time
	Directory               string
	file                    *os.File
	bootcInstallContainerId string
	SkipTLSVerify           bool
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

// GetSize returns the virtual size of the disk in bytes;
// this may be larger than the actual disk usage
func (p *BootcDisk) GetSize() (int64, error) {
	st, err := os.Stat(filepath.Join(p.Directory, config.DiskImage))
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// GetRepoTag returns the repository of the container image
func (p *BootcDisk) GetRepoTag() string {
	return p.RepoTag
}

// GetCreatedAt returns the creation time of the disk image
func (p *BootcDisk) GetCreatedAt() time.Time {
	return p.CreatedAt
}

func (p *BootcDisk) Install(quiet bool, config DiskImageConfig) (err error) {
	p.CreatedAt = time.Now()

	err = p.pullImage(p.SkipTLSVerify)
	if err != nil {
		return
	}

	// Create VM cache dir; one per oci bootc image
	p.Directory = filepath.Join(p.User.CacheDir(), p.ImageId)
	lock := utils.NewCacheLock(p.User.RunDir(), p.Directory)
	locked, err := lock.TryLock(utils.Exclusive)
	if err != nil {
		return fmt.Errorf("error locking the VM cache path: %w", err)
	}
	if !locked {
		return fmt.Errorf("unable to lock the VM cache path")
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			logrus.Errorf("unable to unlock VM %s: %v", p.ImageId, err)
		}
	}()

	if err := os.MkdirAll(p.Directory, os.ModePerm); err != nil {
		return fmt.Errorf("error while making bootc disk directory: %w", err)
	}

	err = p.getOrInstallImageToDisk(quiet, config)
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
func (p *BootcDisk) getOrInstallImageToDisk(quiet bool, diskConfig DiskImageConfig) error {
	diskPath := filepath.Join(p.Directory, config.DiskImage)
	f, err := os.Open(diskPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		logrus.Debugf("No existing disk image found")
		return p.bootcInstallImageToDisk(quiet, diskConfig)
	}
	logrus.Debug("Found existing disk image, comparing digest")
	defer f.Close()
	buf := make([]byte, 4096)
	len, err := unix.Fgetxattr(int(f.Fd()), imageMetaXattr, buf)
	if err != nil {
		// If there's no xattr, just remove it
		os.Remove(diskPath)
		logrus.Debugf("No %s xattr found", imageMetaXattr)
		return p.bootcInstallImageToDisk(quiet, diskConfig)
	}
	bufTrimmed := buf[:len]
	var serializedMeta diskFromContainerMeta
	if err := json.Unmarshal(bufTrimmed, &serializedMeta); err != nil {
		logrus.Warnf("failed to parse serialized meta from %s (%v) %v", diskPath, buf, err)
		return p.bootcInstallImageToDisk(quiet, diskConfig)
	}

	logrus.Debugf("previous disk digest: %s current digest: %s", serializedMeta.ImageDigest, p.ImageId)
	if serializedMeta.ImageDigest == p.ImageId {
		return nil
	}

	return p.bootcInstallImageToDisk(quiet, diskConfig)
}

func align(size int64, align int64) int64 {
	rem := size % align
	if rem != 0 {
		size += (align - rem)
	}
	return size
}

// bootcInstallImageToDisk creates a disk image from a bootc container
func (p *BootcDisk) bootcInstallImageToDisk(quiet bool, diskConfig DiskImageConfig) (err error) {
	fmt.Printf("Executing `bootc install to-disk` from container image %s to create disk image\n", p.RepoTag)
	p.file, err = os.CreateTemp(p.Directory, "podman-bootc-tempdisk")
	if err != nil {
		return err
	}
	size := p.imageData.Size * containerSizeToDiskSizeMultiplier
	if size < diskSizeMinimum {
		size = diskSizeMinimum
	}
	if diskConfig.DiskSize != "" {
		diskConfigSize, err := units.FromHumanSize(diskConfig.DiskSize)
		if err != nil {
			return err
		}
		if size < diskConfigSize {
			size = diskConfigSize
		}
	}
	// Round up to 4k; loopback wants at least 512b alignment
	size = align(size, 4096)
	humanContainerSize := units.HumanSize(float64(p.imageData.Size))
	humanSize := units.HumanSize(float64(size))
	logrus.Infof("container size: %s, disk size: %s", humanContainerSize, humanSize)

	if err := syscall.Ftruncate(int(p.file.Fd()), size); err != nil {
		return err
	}
	logrus.Debugf("Created %s with size %v", p.file.Name(), size)
	doCleanupDisk := true
	defer func() {
		if doCleanupDisk {
			os.Remove(p.file.Name())
		}
	}()

	err = p.runInstallContainer(quiet, diskConfig)
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
func (p *BootcDisk) pullImage(skipTLSVerify bool) error {
	imageData, err := utils.PullAndInspect(p.Ctx, p.ImageNameOrId, skipTLSVerify)
	if err != nil {
		return err
	}

	p.imageData = imageData
	p.ImageId = imageData.ID
	if len(imageData.RepoTags) > 0 {
		p.RepoTag = imageData.RepoTags[0]
	}

	return nil
}

// runInstallContainer runs the bootc installer in a container to create a disk image
func (p *BootcDisk) runInstallContainer(quiet bool, config DiskImageConfig) error {
	c := p.createInstallContainer(config)
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to invoke install: %w", err)
	}
	return nil
}

// createInstallContainer creates a podman command to run the bootc installer.
// Note: This code used to use the Go bindings for the podman remote client, but the
// Attach interface currently leaks goroutines.
func (p *BootcDisk) createInstallContainer(config DiskImageConfig) *exec.Cmd {
	bootcInstallArgs := []string{
		"bootc", "install", "to-disk", "--via-loopback", "--generic-image",
		"--skip-fetch-check",
	}
	if config.Filesystem != "" {
		bootcInstallArgs = append(bootcInstallArgs, "--filesystem", config.Filesystem)
	}
	if config.RootSizeMax != "" {
		bootcInstallArgs = append(bootcInstallArgs, "--root-size="+config.RootSizeMax)
	}
	bootcInstallArgs = append(bootcInstallArgs, "/output/"+filepath.Base(p.file.Name()))

	// Basic config:
	// - force on --remote because we depend on podman machine.
	// - add privileged, pid=host, SELinux config and bind mounts per https://containers.github.io/bootc/bootc-install.html
	// - we need force running as root (i.e., --user=root:root) to overwrite any possible USER directive in the Containerfile
	podmanArgs := []string{"--remote", "run", "--rm", "-i", "--pid=host", "--user=root:root", "--privileged", "--security-opt=label=type:unconfined_t", "--volume=/dev:/dev", "--volume=/var/lib/containers:/var/lib/containers"}
	// Custom bind mounts
	podmanArgs = append(podmanArgs, fmt.Sprintf("--volume=%s:/output", p.Directory))
	if term.IsTerminal(int(os.Stdin.Fd())) {
		podmanArgs = append(podmanArgs, "-t")
	}
	// Other conditional arguments
	if v, ok := os.LookupEnv("BOOTC_INSTALL_LOG"); ok {
		podmanArgs = append(podmanArgs, fmt.Sprintf("--env=RUST_LOG=%s", v))
	}
	// The image name
	podmanArgs = append(podmanArgs, p.ImageNameOrId)
	// And the remaining arguments for bootc install
	podmanArgs = append(podmanArgs, bootcInstallArgs...)

	c := exec.Command("podman", podmanArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}
