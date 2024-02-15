package podman

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// TODO merge with the version in https://github.com/cgwalters/podman/commits/machine-exec/
func NewCommand(args []string) *exec.Cmd {
	c := exec.Command("podman", args...)
	// Default always to using podman machine via the root connection
	c.Env = append(c.Environ(), "CONTAINER_CONNECTION=podman-machine-default-root")
	return c
}

// Run synchronously runs podman as a subprocess, propagating stdout/stderr
func Run(args []string) error {
	c := NewCommand(args)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// GetImage fetches the image if not present, and returns its digest
func GetImage(imageName string) (string, error) {
	// Run an inspect to see if the image is present, otherwise pull.
	// TODO: Add podman pull --if-not-present or so.
	c := NewCommand([]string{"image", "inspect", "-f", "{{.Digest}}", imageName})
	if err := c.Run(); err != nil {
		logrus.Debugf("Inspect failed: %v", err)
		if err := Run([]string{"pull", imageName}); err != nil {
			return "", fmt.Errorf("pulling image: %w", err)
		}
	}
	c = NewCommand([]string{"image", "inspect", "-f", "{{.Digest}}", imageName})
	buf := &bytes.Buffer{}
	c.Stdout = buf
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("failed to inspect %s: %w", imageName, err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// Get the SSH key podman machine generates by default
func MachineSSHKey() (string, string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	privkey := filepath.Join(homedir, ".ssh/podman-machine-default")
	pubkey := privkey + ".pub"
	return privkey, pubkey, nil
}
