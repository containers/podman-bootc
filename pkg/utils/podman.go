package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

type MachineContext struct {
	Ctx             context.Context
	SSHIdentityPath string
}

type machineInfo struct {
	podmanSocket    string
	sshIdentityPath string
	rootful         bool
}

// PullAndInspect inpects the image, pulling in if the image if required
func PullAndInspect(ctx context.Context, imageNameOrId string, skipTLSVerify bool) (*types.ImageInspectReport, error) {
	pullPolicy := "missing"
	_, err := images.Pull(ctx, imageNameOrId, &images.PullOptions{Policy: &pullPolicy, SkipTLSVerify: &skipTLSVerify})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	imageInfo, err := images.GetImage(ctx, imageNameOrId, &images.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	return imageInfo, nil
}

func GetMachineContext() (*MachineContext, error) {
	//podman machine connection
	machineInfo, err := getMachineInfo()
	if err != nil {
		return nil, fmt.Errorf("unable to get podman machine info: %w", err)
	}

	if machineInfo == nil {
		return nil, errors.New("rootful podman machine is required, please run 'podman machine init --rootful'")
	}

	if !machineInfo.rootful {
		return nil, errors.New("rootful podman machine is required, please run 'podman machine set --rootful'")
	}

	if _, err := os.Stat(machineInfo.podmanSocket); err != nil {
		return nil, fmt.Errorf("podman machine socket is missing: %w", err)
	}

	ctx, err := bindings.NewConnectionWithIdentity(
		context.Background(),
		fmt.Sprintf("unix://%s", machineInfo.podmanSocket),
		machineInfo.sshIdentityPath,
		true)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the podman socket: %w", err)
	}

	mc := &MachineContext{
		Ctx:             ctx,
		SSHIdentityPath: machineInfo.sshIdentityPath,
	}
	return mc, nil
}

func getMachineInfo() (*machineInfo, error) {
	minfo, err := getPv5MachineInfo()
	if err != nil {
		var errIncompatibleMachineConfig *define.ErrIncompatibleMachineConfig
		var errVMDoesNotExist *define.ErrVMDoesNotExist
		if errors.As(err, &errIncompatibleMachineConfig) || errors.As(err, &errVMDoesNotExist) {
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
func getPv5MachineInfo() (*machineInfo, error) {
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

	pmi := machineInfo{
		podmanSocket:    podmanSocket.GetPath(),
		sshIdentityPath: pm.SSH.IdentityPath,
		rootful:         pm.HostUser.Rootful,
	}
	return &pmi, nil
}

// Just to support podman v4.9, it will be removed in the future
func getPv4MachineInfo() (*machineInfo, error) {
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

	return &machineInfo{
		podmanSocket:    machineInspect[0].ConnectionInfo.PodmanSocket.Path,
		sshIdentityPath: machineInspect[0].SSHConfig.IdentityPath,
		rootful:         machineInspect[0].Rootful,
	}, nil
}
