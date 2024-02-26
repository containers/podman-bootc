package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func CommonSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "StrictHostKeyChecking=no", "-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
