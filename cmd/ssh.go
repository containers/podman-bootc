package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:     "ssh <ID>",
	Short:   "SSH into an existing OS Container machine",
	Long:    "SSH into an existing OS Container machine",
	Args:    cobra.MinimumNArgs(1),
	Example: `podman bootc ssh 6c6c2fc015fe`,
	Run:     ssh,
}
var sshUser string

func init() {
	RootCmd.AddCommand(sshCmd)
	sshCmd.Flags().StringVarP(&sshUser, "user", "u", "root", "--user <user name> (default: root)")
}

func ssh(_ *cobra.Command, args []string) {
	err := doSsh(args)
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doSsh(args []string) error {
	id := args[0]
	cfg, err := loadConfig(id)
	if err != nil {
		return err
	}

	cmd := make([]string, 0)
	if len(args) > 1 {
		cmd = args[1:]
	}

	return CommonSSH(sshUser, cfg.SshIdentity, id, cfg.SshPort, cmd)
}

func CommonSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "StrictHostKeyChecking=no", "-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
