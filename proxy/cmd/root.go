package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/containers/podman-bootc/pkg/vsock"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type mode string

const (
	unixToVsock mode = "unixToVsock"
	vsockToUnix mode = "vsockToUnix"
)

func (m *mode) String() string {
	return string(*m)
}

func (m *mode) Set(val string) error {
	switch val {
	case string(vsockToUnix), string(unixToVsock):
		*m = mode(val)
		return nil
	default:
		return fmt.Errorf("invalid mode: %s (must be '%s' or '%s')", val, unixToVsock, vsockToUnix)
	}
}

func (m *mode) Type() string {
	return "mode"
}

type rootCmd struct {
	proxy      *vsock.Proxy
	logLevel   string
	listenMode mode
	cid        uint32
	port       uint32
	socket     string
}

func NewRootCmd() *cobra.Command {
	c := rootCmd{}
	cmd := &cobra.Command{
		Use:               "proxy",
		Short:             "Proxy connections between VSOCK and UNIX socket",
		Long:              "Proxy the connection between VSOCK and UNIX socket based on the direction",
		PersistentPreRunE: c.preExec,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.run()
		},
	}

	cmd.PersistentFlags().Uint32VarP(&c.cid, "cid", "c", 0, "CID allocated by the VM")
	cmd.PersistentFlags().Uint32VarP(&c.port, "port", "p", 0, "Port for the VSOCK on the VM")
	cmd.PersistentFlags().StringVarP(&c.socket, "socket", "s", "", "Socket for the proxy")
	cmd.PersistentFlags().StringVarP(&c.logLevel, "log-level", "", "", "Set log level")
	cmd.PersistentFlags().VarP(&c.listenMode, "listen-mode", "l",
		fmt.Sprintf("Direction for the listentin proxy, values: %s or %s", unixToVsock, vsockToUnix))
	cmd.MarkPersistentFlagRequired("port")
	cmd.MarkPersistentFlagRequired("socket")
	cmd.MarkPersistentFlagRequired("listen-mode")

	return cmd
}

func (c *rootCmd) preExec(cmd *cobra.Command, args []string) error {
	if c.logLevel != "" {
		level, err := log.ParseLevel(c.logLevel)
		if err != nil {
			return err
		}
		log.SetLevel(level)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	socket, _ := cmd.Flags().GetString("socket")
	if socket == "" {
		return fmt.Errorf("the socket needs to be set")
	}

	return nil
}

func (c *rootCmd) validateArgs() error {
	if c.port == 0 {
		return fmt.Errorf("the port cannot be 0")
	}
	if c.listenMode == unixToVsock && c.cid == 0 {
		return fmt.Errorf("the cid cannot be 0 when the listen mode is unixToVsock")
	}

	return nil
}

func (c *rootCmd) run() error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	switch c.listenMode {
	case vsockToUnix:
		c.proxy = vsock.NewProxyVSockToUnixSocket(c.port, c.socket)
	case unixToVsock:
		c.proxy = vsock.NewProxyUnixSocketToVsock(c.port, c.cid, c.socket)
	}

	if err := c.proxy.Start(); err != nil {
		return err
	}
	defer c.proxy.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	return nil
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
