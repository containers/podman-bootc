package utils

import (
	"errors"
	"fmt"
	"path/filepath"
	"podman-bootc/pkg/config"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

type MachineInfo struct {
	PodmanSocket    string
	SSHIdentityPath string
	Rootful         bool
}

func GetMachineInfo() (*MachineInfo, error) {
	minfo, err := getMachineInfo()
	if err != nil {
		var errIncompatibleMachineConfig *define.ErrIncompatibleMachineConfig
		if errors.As(err, &errIncompatibleMachineConfig) {
			minfo, err := getPv4MachineInfo()
			if err != nil {
				return nil, err
			}
			return minfo, nil
		}
		return nil, err
	}

	return minfo, nil
}

// Get podman v5 machine info
func getMachineInfo() (*MachineInfo, error) {
	prov, err := provider.Get()
	if err != nil {
		return nil, fmt.Errorf("getting podman machine provider: %w", err)
	}

	dirs, err := env.GetMachineDirs(prov.VMType())
	if err != nil {
		return nil, fmt.Errorf("getting podman machine dirs: %w", err)
	}

	pm, err := vmconfigs.LoadMachineByName(machine.DefaultMachineName, dirs)
	if err != nil {
		return nil, fmt.Errorf("load podman machine info: %w", err)
	}

	podmanSocket, _, err := pm.ConnectionInfo(prov.VMType())
	if err != nil {
		return nil, fmt.Errorf("getting podman machine connection info: %w", err)
	}

	pmi := MachineInfo{
		PodmanSocket:    podmanSocket.GetPath(),
		SSHIdentityPath: pm.SSH.IdentityPath,
		Rootful:         pm.HostUser.Rootful,
	}
	return &pmi, nil
}

// Just to support podman v4.9, it will be removed in the future
func getPv4MachineInfo() (*MachineInfo, error) {
	// Let's cheat and use hard-coded values for podman v4.
	// We do that because libpod doesn't work if we import both v4 and v5.
	return &MachineInfo{
		PodmanSocket:    filepath.Join(config.User.HomeDir, ".local/share/containers/podman/machine/qemu/podman.sock"),
		SSHIdentityPath: filepath.Join(config.UserSshDir, "podman-machine-default"),
		Rootful:         true,
	}, nil
}
