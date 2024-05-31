package e2e

import (
	"path/filepath"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"
)

func pidFilePath(id string) (pidFilePath string, err error) {
	user, err := user.NewUser()
	if err != nil {
		return
	}

	_, cacheDir, err := vm.GetVMCachePath(id, user)
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
