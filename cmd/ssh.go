package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:     "ssh NAME",
	Short:   "SSH into an existing OS Container machine",
	Long:    "SSH into an existing OS Container machine",
	Args:    cobra.MinimumNArgs(1),
	Example: `osc ssh fedora-base`,
	RunE:    ssh,
}

func init() {
	RootCmd.AddCommand(sshCmd)
}

func ssh(_ *cobra.Command, args []string) error {
	name := args[0]
	runCfg, err := LoadRunningVmFromDisk(name)
	if err != nil {
		return err
	}

	vm := NewVMPartial(name)

	cmd := make([]string, 0)
	if len(args) > 1 {
		cmd = args[1:]
	}

	return CommonSSH("root", vm.SshPriKey, name, int(runCfg.SshPort), cmd)
}

func CommonSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@localhost"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
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
