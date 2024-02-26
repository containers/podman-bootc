package cmd

import (
	"fmt"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/ssh"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop ID",
	Short: "Stop an existing OS Container machine",
	Long:  "Stop an existing OS Container machine",
	Args:  cobra.ExactArgs(1),
	Run:   stopVm,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func stopVm(_ *cobra.Command, args []string) {
	err := doStopVm(args[0])
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doStopVm(id string) error {

	cfg, err := config.LoadConfig(id)
	if err != nil {
		return err
	}

	poweroff := []string{"poweroff"}
	return ssh.CommonSSH("root", cfg.SshIdentity, id, cfg.SshPort, poweroff)
}
