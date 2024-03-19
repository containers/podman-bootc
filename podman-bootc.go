package main

import (
	"os"
	"os/signal"
	"syscall"

	"podman-bootc/cmd"
	"podman-bootc/pkg/bootc"
	"podman-bootc/pkg/utils"

	"github.com/sirupsen/logrus"
)

func cleanup() {
	machineInfo, err := utils.GetMachineInfo()
	if err != nil {
		logrus.Errorf("unable to get podman machine info: %s", err)
		os.Exit(0)
	}

	if err := bootc.NewBootcDisk("", machineInfo).Cleanup(); err != nil {
		logrus.Errorf("unable to cleanup bootc image: %s", err)
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
