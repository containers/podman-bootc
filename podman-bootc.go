package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/bootc-org/podman-bootc/cmd"
	"gitlab.com/bootc-org/podman-bootc/pkg/bootc"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
	"gitlab.com/bootc-org/podman-bootc/pkg/utils"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/sirupsen/logrus"
)

func cleanup() {
	user, err := user.NewUser()
	if err != nil {
		logrus.Errorf("unable to get user info: %s", err)
		os.Exit(0)
	}

	//podman machine connection
	machineInfo, err := utils.GetMachineInfo(user)
	if err != nil {
		logrus.Errorf("unable to get podman machine info: %s", err)
		os.Exit(1)
	}

	if machineInfo == nil {
		logrus.Errorf("rootful podman machine is required, please run 'podman machine init --rootful'")
		os.Exit(1)
	}

	if !machineInfo.Rootful {
		logrus.Errorf("rootful podman machine is required, please run 'podman machine set --rootful'")
		os.Exit(1)
	}

	if _, err := os.Stat(machineInfo.PodmanSocket); err != nil {
		logrus.Errorf("podman machine socket is missing. Is podman machine running?\n%s", err)
		os.Exit(1)
	}

	ctx, err := bindings.NewConnectionWithIdentity(
		context.Background(),
		fmt.Sprintf("unix://%s", machineInfo.PodmanSocket),
		machineInfo.SSHIdentityPath,
		true)
	if err != nil {
		println(utils.PodmanMachineErrorMessage)
		logrus.Errorf("failed to connect to the podman socket. Is podman machine running?\n%s", err)
		os.Exit(1)
	}

	//delete the disk image
	err = bootc.NewBootcDisk("", ctx, user).Cleanup()
	if err != nil {
		logrus.Errorf("unable to get podman machine info: %s", err)
		os.Exit(0)
	}
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		cleanup()
		os.Exit(1)
	}()

	cmd.Execute()
	os.Exit(cmd.ExitCode)
}
