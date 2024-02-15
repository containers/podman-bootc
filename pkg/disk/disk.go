package disk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"podmanbootc/pkg/config"
	"podmanbootc/pkg/podman"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const diskSize = 10 * 1024 * 1024 * 1024

// imageMetaXattr holds serialized diskFromContainerMeta
const imageMetaXattr = "user.bootc.meta"

// diskFromContainerMeta is serialized to JSON in a user xattr on a disk image
type diskFromContainerMeta struct {
	// imageDigest is the digested sha256 of the container that was used to build this disk
	ImageDigest string `json:"imageDigest"`
}

// InstallImage generates a disk image from the provided container image
func InstallImage(vmdir, containerImage, imageDigest string) error {
	temporaryDisk, err := os.CreateTemp(vmdir, "podman-bootc-tempdisk")
	if err != nil {
		return err
	}
	if err := syscall.Ftruncate(int(temporaryDisk.Fd()), diskSize); err != nil {
		return err
	}
	doCleanupDisk := true
	defer func() {
		if doCleanupDisk {
			os.Remove(temporaryDisk.Name())
		}
	}()

	// https://github.com/containers/bootc/blob/main/docs/install.md#using-bootc-install-to-disk---via-loopback
	volumeBind := fmt.Sprintf("%s:/output", vmdir)
	installArgsForPodman := []string{"run", "--rm", "--privileged", "--pid=host", "-v", volumeBind, "--security-opt", "label=type:unconfined_t"}
	if val, ok := os.LookupEnv("PODMAN_BOOTC_INST_ARGS"); ok {
		parts := strings.Split(val, " ")
		installArgsForPodman = append(installArgsForPodman, parts...)
	}
	installArgsForPodman = append(installArgsForPodman, containerImage)
	installArgsForBootc := []string{"bootc", "install", "to-disk", "--via-loopback", "--generic-image", "--skip-fetch-check", "/output/" + filepath.Base(temporaryDisk.Name())}
	if err := podman.PodmanRecurseRun(append(installArgsForPodman, installArgsForBootc...)); err != nil {
		return fmt.Errorf("failed to generate disk image via bootc install to-disk --via-loopback")
	}
	serializedMeta := diskFromContainerMeta{
		ImageDigest: imageDigest,
	}
	buf, err := json.Marshal(serializedMeta)
	if err != nil {
		return err
	}
	if err := unix.Fsetxattr(int(temporaryDisk.Fd()), imageMetaXattr, buf, 0); err != nil {
		return fmt.Errorf("failed to set xattr: %w", err)
	}
	diskPath := filepath.Join(vmdir, config.BootcDiskImage)

	if err := os.Rename(temporaryDisk.Name(), diskPath); err != nil {
		return fmt.Errorf("failed to rename to %s: %w", diskPath, err)
	}
	doCleanupDisk = false

	return nil
}

func GetOrInstallImage(vmdir, containerImage, imageDigest string) error {
	diskPath := filepath.Join(vmdir, config.BootcDiskImage)
	f, err := os.Open(diskPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return InstallImage(vmdir, containerImage, imageDigest)
	}
	defer f.Close()
	buf := make([]byte, 4096)
	len, err := unix.Fgetxattr(int(f.Fd()), imageMetaXattr, buf)
	if err != nil {
		// If there's no xattr, just remove it
		os.Remove(diskPath)
		return InstallImage(vmdir, containerImage, imageDigest)
	}
	bufTrimmed := buf[:len]
	var serializedMeta diskFromContainerMeta
	if err := json.Unmarshal(bufTrimmed, &serializedMeta); err != nil {
		logrus.Warnf("failed to parse serialized meta from %s (%v) %v", diskPath, buf, err)
		return InstallImage(vmdir, containerImage, imageDigest)
	}

	logrus.Debugf("previous disk digest: %s current digest: %s", serializedMeta.ImageDigest, imageDigest)
	if serializedMeta.ImageDigest == imageDigest {
		return nil
	}

	return InstallImage(vmdir, containerImage, imageDigest)
}
