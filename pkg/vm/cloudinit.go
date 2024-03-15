package vm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"podman-bootc/pkg/config"
)

func (b *BootcVMCommon) ParseCloudInit() (err error) {
	// cloud-init required?
	if b.hasCloudInit {
		if b.cloudInitDir == "" {
			return errors.New("empty cloud init directory")
		}

		path := b.getPath()
		err = b.createCiDataIso(path)
		if err != nil {
			return fmt.Errorf("creating cloud-init iso: %w", err)
		}

		ciDataIso := filepath.Join(b.directory, config.CiDataIso)
		b.cloudInitArgs = ciDataIso
	}

	return nil
}

func (b *BootcVMCommon) getPath() string {
	if strings.Contains(b.cloudInitDir, ":") {
		return b.cloudInitDir[strings.IndexByte(b.cloudInitDir, ':')+1:]
	}
	return b.cloudInitDir
}

func (b *BootcVMCommon) createCiDataIso(inDir string) error {
	isoOutFile := filepath.Join(b.directory, config.CiDataIso)

	args := []string{"-output", isoOutFile}
	args = append(args, "-volid", "cidata", "-joliet", "-rock", "-partition_cyl_align", "on")
	args = append(args, inDir)

	cmd := exec.Command("xorrisofs", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
