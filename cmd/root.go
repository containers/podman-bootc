package cmd

import (
	"os"

	"gitlab.com/bootc-org/podman-bootc/pkg/user"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var (
	RootCmd = &cobra.Command{
		Use:               "podman-bootc",
		Short:             "Run bootable containers as a virtual machine",
		Long:              "Run bootable containers as a virtual machine",
		PersistentPreRunE: preExec,
		SilenceUsage:      true,
	}
	ExitCode int
)

var rootLogLevel string

func preExec(cmd *cobra.Command, args []string) error {
	if rootLogLevel != "" {
		level, err := logrus.ParseLevel(rootLogLevel)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)
	}

	user, err := user.NewUser()
	if err != nil {
		return err
	}

	if err := user.InitOSCDirs(); err != nil {
		return err
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	logrus.SetLevel(logrus.WarnLevel)
	RootCmd.PersistentFlags().StringVarP(&rootLogLevel, "log-level", "", "", "Set log level")
}
