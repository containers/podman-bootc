package credentials

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"podman-bootc/pkg/config"
)

// Generatekeys creates an ed25519 set of keys
func Generatekeys(outputDir string) (string, error) {
	sshIdentity := filepath.Join(outputDir, config.SshKeyFile)
	_ = os.Remove(sshIdentity)
	_ = os.Remove(sshIdentity + ".pub")

	args := []string{"-N", "", "-t", "ed25519", "-f", sshIdentity}
	cmd := exec.Command("ssh-keygen", args...)
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("ssh key generation: redirecting stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ssh key generation: executing ssh-keygen: %w", err)
	}

	waitErr := cmd.Wait()
	if waitErr == nil {
		return sshIdentity, nil
	}

	errMsg, err := io.ReadAll(stdErr)
	if err != nil {
		return "", fmt.Errorf("ssh key generation, unable to read from stderr: %w", waitErr)
	}

	return "", fmt.Errorf("failed to generate ssh keys: %s: %w", string(errMsg), waitErr)
}
