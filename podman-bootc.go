package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/containers/podman-bootc/cmd"
	"github.com/containers/podman-bootc/pkg/bootc"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/sirupsen/logrus"
)

func cleanup() {
	user, err := user.NewUser()
	if err != nil {
		logrus.Errorf("unable to get user info: %s", err)
		os.Exit(0)
	}

	machine, err := utils.GetMachineContext()
	if err != nil {
		println(utils.PodmanMachineErrorMessage)
		logrus.Errorf("failed to connect to podman machine. Is podman machine running?\n%s", err)
		os.Exit(1)
	}

	//delete the disk image
	err = bootc.NewBootcDisk("", machine.Ctx, user).Cleanup()
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
