package main

import (
	"os"
	"os/signal"
	"podman-bootc/cmd"
	"podman-bootc/pkg/bootc"
	"syscall"

	"github.com/sirupsen/logrus"
)

func cleanup() {
	err := bootc.NewBootcDisk("").Cleanup()
	if err != nil {
		logrus.Errorf("unable to cleanup bootc image: %s", err)
		os.Exit(0)
	}
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func(){
			<-c
			cleanup()
			os.Exit(1)
	}()

	cmd.Execute()
}
