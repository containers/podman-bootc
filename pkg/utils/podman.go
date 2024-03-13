package utils

import (
	"errors"
	"fmt"
	"podman-bootc/pkg/user"

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

func GetMachineInfo(user user.User) (*MachineInfo, error) {
	minfo, err := getMachineInfo()
	if err != nil {
		var errIncompatibleMachineConfig *define.ErrIncompatibleMachineConfig
		var errVMDoesNotExist *define.ErrVMDoesNotExist
		if errors.As(err, &errIncompatibleMachineConfig) || errors.As(err, &errVMDoesNotExist) {
			minfo, err := getPv4MachineInfo(user)
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
func getPv4MachineInfo(user user.User) (*MachineInfo, error) {
	// Let's cheat and use hard-coded values for podman v4.
	// We do that because libpod doesn't work if we import both v4 and v5.
	return &MachineInfo{
		PodmanSocket:    user.MachineSocket(),
		SSHIdentityPath: user.MachineSshKeyPriv(),
		Rootful:         true,
	}, nil
}
