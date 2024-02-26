package cmd

import (
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/ssh"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <ID>",
	Short: "SSH into an existing OS Container machine",
	Long:  "SSH into an existing OS Container machine",
	Args:  cobra.MinimumNArgs(1),
	RunE:  doSsh,
}
var sshUser string

func init() {
	RootCmd.AddCommand(sshCmd)
	sshCmd.Flags().StringVarP(&sshUser, "user", "u", "root", "--user <user name> (default: root)")
}

func doSsh(_ *cobra.Command, args []string) error {
	id := args[0]
	cfg, err := config.LoadConfig(id)
	if err != nil {
		return err
	}

	cmd := make([]string, 0)
	if len(args) > 1 {
		cmd = args[1:]
	}

	return ssh.CommonSSH(sshUser, cfg.SshIdentity, id, cfg.SshPort, cmd)
}
