package vm

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func OemString(user, pubKeyFile string) (string, error) {
	tmpFilesCmd, err := tmpFileInjectSshKeyEnc(user, pubKeyFile+".pub")
	if err != nil {
		return "", err
	}
	oemString := fmt.Sprintf("type=11,value=io.systemd.credential.binary:tmpfiles.extra=%s", tmpFilesCmd)
	return oemString, nil
}

func tmpFileInjectSshKeyEnc(user, pubKeyFile string) (string, error) {
	pubKey, err := os.ReadFile(pubKeyFile)
	if err != nil {
		return "", err
	}
	pubKeyEnc := base64.StdEncoding.EncodeToString(pubKey)

	userHomeDir := "/root"
	if user != "root" {
		userHomeDir = filepath.Join("/home", user)
	}

	tmpFileCmd := fmt.Sprintf("d %[1]s/.ssh 0750 %[2]s %[2]s -\nf+~ %[1]s/.ssh/authorized_keys 700 %[2]s %[2]s - %[3]s", userHomeDir, user, pubKeyEnc)

	tmpFileCmdEnc := base64.StdEncoding.EncodeToString([]byte(tmpFileCmd))
	return tmpFileCmdEnc, nil
}
