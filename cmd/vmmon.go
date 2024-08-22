//go:build darwin

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/vm"

	"github.com/spf13/cobra"
)

// We treat this command as internal, we don't expect the user to call
// this directly. In the future this will be replaced by an external binary
var (
	monCmd = &cobra.Command{
		Use:    "vmmon <ID> <username> <ssh identity> <ssh port>",
		Hidden: true,
		Args:   cobra.ExactArgs(4),
		RunE:   doMon,
	}
	console bool
)

func init() {
	RootCmd.AddCommand(monCmd)
	runCmd.Flags().BoolVar(&console, "console", false, "Show boot console")
}

func doMon(_ *cobra.Command, args []string) error {
	usr, err := user.NewUser()
	if err != nil {
		return err
	}

	ctx := context.Background()
	fullImageId := args[0]
	username := args[1]
	sshIdentity := args[2]

	sshPort, err := strconv.Atoi(args[3])
	if err != nil {
		return fmt.Errorf("invalid ssh port: %w", err)
	}

	cacheDir := filepath.Join(usr.CacheDir(), fullImageId)

	// MacOS has a 104 bytes limit for a unix socket path
	runDir := filepath.Join(usr.RunDir(), fullImageId[0:12])
	if err := os.MkdirAll(runDir, os.ModePerm); err != nil {
		return err
	}

	params := vm.MonitorParmeters{
		CacheDir:    cacheDir,
		RunDir:      runDir,
		Username:    username,
		SshIdentity: sshIdentity,
		SshPort:     sshPort,
	}

	return vm.StartMonitor(ctx, params)
}
