package main

import (
	"os"
	"os/signal"
	"syscall"

	"podman-bootc/cmd"
	"podman-bootc/pkg/bootc"
	"podman-bootc/pkg/utils"

	"podman-bootc/pkg/user"

	"github.com/sirupsen/logrus"
)

func cleanup() {
	user, err := user.NewUser()
	if err != nil {
		logrus.Errorf("unable to get user info: %s", err)
		os.Exit(0)
	}

	machineInfo, err := utils.GetMachineInfo(user)
	if err != nil {
		logrus.Errorf("unable to get podman machine info: %s", err)
		os.Exit(0)
	}

	err = bootc.NewBootcDisk("", machineInfo, user).Cleanup()
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
}
