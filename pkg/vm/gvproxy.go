package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman-bootc/pkg/utils"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/sirupsen/logrus"
)

const gvproxyBinaryName = "gvproxy"

type Vmm int

const (
	pidFileName = "gvproxy.pid"
	socketFile  = "net.sock"
	// How log we should wait for gvproxy to be ready
	maxBackoffs = 5
	backoff     = time.Millisecond * 200
)

type gvproxyParams struct {
	SshPort int
}

type gvproxyDaemon struct {
	socketPath string
	pidFile    string
	cmd        *exec.Cmd
}

func newGvproxy(ctx context.Context, binaryPath, rundir string, param gvproxyParams) *gvproxyDaemon {
	socketPath := filepath.Join(rundir, socketFile)
	pidFile := filepath.Join(rundir, pidFileName)

	gvpCmd := gvproxy.NewGvproxyCommand()
	gvpCmd.SSHPort = param.SshPort
	gvpCmd.PidFile = pidFile
	gvpCmd.AddVfkitSocket(fmt.Sprintf("unixgram://%s", socketPath))

	cmdLine := gvpCmd.ToCmdline()
	cmd := exec.CommandContext(ctx, binaryPath, cmdLine...)
	logrus.Debugf("gvproxy command-line: %s %s", binaryPath, strings.Join(cmdLine, " "))

	return &gvproxyDaemon{socketPath: socketPath, pidFile: pidFile, cmd: cmd}
}

// Start spawn the gvproxy daemon, killing any running daemon using the same unix socket file.
// It blocks until the unix socket file exists, this does not guarantee that the socket will be
// on listen state
func (d *gvproxyDaemon) start() error {
	cleanup(d.pidFile, d.socketPath)

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("unable to start gvproxy: %w", err)
	}

	// this is racy, the socket file could exist but not be in the listen state yet
	if err := utils.WaitForFileWithBackoffs(maxBackoffs, backoff, d.socketPath); err != nil {
		return fmt.Errorf("waiting for gvproxy socket: %w", err)
	}
	return nil
}

func (d *gvproxyDaemon) stop() error {
	return d.cmd.Cancel()
}

func (d *gvproxyDaemon) wait() error {
	return d.cmd.Wait()
}

func cleanup(pidFile, socketPath string) {
	// Let's kill any possible running daemon
	pid, err := utils.ReadPidFile(pidFile)
	if err == nil {
		_ = utils.SendInterrupt(pid)
	}

	_ = os.Remove(pidFile)
	_ = os.Remove(socketPath)
}
