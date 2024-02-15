package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateSSHKey generates a SSH key
func CreateSSHKey() ([]byte, []byte, error) {
	var err error
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, nil, err
	}
	sshKeyPath := filepath.Join(tmpd, "ssh.key")
	sshPubKeyPath := sshKeyPath + ".pub"
	c := exec.Command("ssh-keygen", "-N", "", "-t", "ed25519", "-f", sshKeyPath)
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return nil, nil, fmt.Errorf("running ssh-keygen: %w", err)
	}
	pubkeyBuf, err := os.ReadFile(sshPubKeyPath)
	if err != nil {
		return nil, nil, err
	}
	privkeyBuf, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return nil, nil, err
	}
	return pubkeyBuf, privkeyBuf, nil
}
