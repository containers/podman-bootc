package vm

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func oemStringSystemdCredential(username, sshIdentity string) (string, error) {
	tmpFilesCmd, err := tmpFileSshKey(username, sshIdentity)
	if err != nil {
		return "", err
	}
	oemString := fmt.Sprintf("io.systemd.credential.binary:tmpfiles.extra=%s", tmpFilesCmd)
	return oemString, nil
}

func tmpFileSshKey(username, sshIdentity string) (string, error) {
	pubKey, err := os.ReadFile(sshIdentity + ".pub")
	if err != nil {
		return "", err
	}
	pubKeyEnc := base64.StdEncoding.EncodeToString(pubKey)

	userHomeDir := "/root"
	if username != "root" {
		userHomeDir = filepath.Join("/home", username)
	}

	tmpFileCmd := fmt.Sprintf("d %[1]s/.ssh 0750 %[2]s %[2]s -\nf+~ %[1]s/.ssh/authorized_keys 700 %[2]s %[2]s - %[3]s", userHomeDir, username, pubKeyEnc)

	tmpFileCmdEnc := base64.StdEncoding.EncodeToString([]byte(tmpFileCmd))
	return tmpFileCmdEnc, nil
}
