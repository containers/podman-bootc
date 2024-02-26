package cmd

import (
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/ssh"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop ID",
	Short: "Stop an existing OS Container machine",
	Long:  "Stop an existing OS Container machine",
	Args:  cobra.ExactArgs(1),
	RunE:  doStop,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func doStop(_ *cobra.Command, args []string) error {
	id := args[0]
	cfg, err := config.LoadConfig(id)
	if err != nil {
		return err
	}

	poweroff := []string{"poweroff"}
	return ssh.CommonSSH("root", cfg.SshIdentity, id, cfg.SshPort, poweroff)
}
