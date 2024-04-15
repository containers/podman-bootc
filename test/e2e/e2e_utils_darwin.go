package e2e

import (
	"path/filepath"

	"gitlab.com/bootc-org/podman-bootc/pkg/config"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
	"gitlab.com/bootc-org/podman-bootc/pkg/utils"
	"gitlab.com/bootc-org/podman-bootc/pkg/vm"
)

func pidFilePath(id string) (pidFilePath string, err error) {
	user, err := user.NewUser()
	if err != nil {
		return
	}

	cacheDir, err := vm.GetVMCachePath(id, user)
	if err != nil {
		return
	}

	return filepath.Join(cacheDir, config.RunPidFile), nil
}

func VMExists(id string) (exits bool, err error) {
	pidFilePath, err := pidFilePath(id)
	if err != nil {
		return false, err
	}
	return utils.FileExists(pidFilePath)
}

func VMIsRunning(id string) (exits bool, err error) {
	pidFilePath, err := pidFilePath(id)
	if err != nil {
		return false, err
	}

	pid, err := utils.ReadPidFile(pidFilePath)
	if err != nil {
		return false, err
	}

	if pid != -1 && utils.IsProcessAlive(pid) {
		return true, nil
	} else {
		return false, nil
	}
}
