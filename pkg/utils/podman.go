package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"gitlab.com/bootc-org/podman-bootc/pkg/user"

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
	//check if a default podman machine exists
	listCmd := exec.Command("podman", "machine", "list", "--format", "json")
	var listCmdOutput strings.Builder
	listCmd.Stdout = &listCmdOutput
	err := listCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list podman machines: %w", err)
	}
	var machineList []MachineList
	err = json.Unmarshal([]byte(listCmdOutput.String()), &machineList)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal podman machine inspect output: %w", err)
	}

	var defaultMachineName string
	if len(machineList) == 0 {
		return nil, errors.New("no podman machine found")
	} else if len(machineList) == 1 {
		// if there is only one machine, use it as the default
		// afaict, podman will use a single machine as the default, even if Default is false
		// in the output of `podman machine list`
		if !machineList[0].Running {
			println(PodmanMachineErrorMessage)
			return nil, errors.New("the default podman machine is not running")
		}
		defaultMachineName = machineList[0].Name
	} else {
		foundDefaultMachine := false
		for _, machine := range machineList {
			if machine.Default {
				if !machine.Running {
					println(PodmanMachineErrorMessage)
					return nil, errors.New("the default podman machine is not running")
				}

				foundDefaultMachine = true
				defaultMachineName = machine.Name
			}
		}

		if !foundDefaultMachine {
			println(PodmanMachineErrorMessage)
			return nil, errors.New("a default podman machine is not running")
		}
	}

	// check if the default podman machine is rootful
	inspectCmd := exec.Command("podman", "machine", "inspect", defaultMachineName)
	var inspectCmdOutput strings.Builder
	inspectCmd.Stdout = &inspectCmdOutput
	err = inspectCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect podman machine: %w", err)
	}

	var machineInspect []MachineInspect
	err = json.Unmarshal([]byte(inspectCmdOutput.String()), &machineInspect)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal podman machine inspect output: %w", err)
	}

	if len(machineInspect) == 0 {
		return nil, errors.New("no podman machine found")
	}

	return &MachineInfo{
		PodmanSocket:    machineInspect[0].ConnectionInfo.PodmanSocket.Path,
		SSHIdentityPath: machineInspect[0].SSHConfig.IdentityPath,
		Rootful:         machineInspect[0].Rootful,
	}, nil
}
