package vm

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/containers/podman-bootc/pkg/config"

	"github.com/sirupsen/logrus"
)

type stopFunction func() error
type waitFunction func() error

type MonitorParmeters struct {
	CacheDir    string
	RunDir      string
	Username    string
	SshIdentity string
	SshPort     int
}

func StartMonitor(ctx context.Context, params MonitorParmeters) error {
	netSocket, stopGvpd, err := startNetworkDaemon(ctx, params.RunDir, params.SshPort)
	if err != nil {
		return err
	}
	defer func() {
		if err := stopGvpd(); err != nil {
			logrus.Errorf("stoping gvproxy: %v", err)
		}
	}()

	krkWait, err := startKrunkit(ctx, params.CacheDir, params.Username, params.SshIdentity, netSocket)
	if err != nil {
		return err
	}

	if err := krkWait(); err != nil {
		logrus.Debugf("krunkit wait return error: %v", err)
	}

	return nil
}

func startNetworkDaemon(ctx context.Context, runDir string, sshPort int) (string, stopFunction, error) {
	binaryPath, err := getBinaryPath(gvproxyBinaryName)
	if err != nil {
		return "", nil, err
	}

	params := gvproxyParams{
		SshPort: sshPort,
	}

	daemon := newGvproxy(ctx, binaryPath, runDir, params)

	if err := daemon.start(); err != nil {
		return "", nil, fmt.Errorf("could not start %s: %w", binaryPath, err)
	}

	// the only purpose of this gorutine is to capture the signal from the child process
	go func(ctx context.Context, daemon *gvproxyDaemon) {
		if err := daemon.wait(); err != nil {
			logrus.Debugf("gvproxy wait return error: %v", err)
		}
	}(ctx, daemon)

	return daemon.socketPath, daemon.stop, nil
}

func startKrunkit(ctx context.Context, cacheDir, username, sshIdentity, netSocketPath string) (waitFunction, error) {
	binaryPath, err := getBinaryPath(krunkitBinaryName)
	if err != nil {
		return nil, err
	}

	pidFile := filepath.Join(cacheDir, config.RunPidFile)
	disk := filepath.Join(cacheDir, config.DiskImage)

	oemString, err := oemStringSystemdCredential(username, sshIdentity)
	if err != nil {
		return nil, fmt.Errorf("creating oemstring systemd credential %w", err)
	}

	params := krunkitParams{
		disk:      disk,
		netSocket: netSocketPath,
		oemString: oemString,
		pidFile:   pidFile,
	}

	krk := newKrunkit(ctx, binaryPath, params)

	err = krk.start()
	if err != nil {
		return nil, err
	}

	return krk.wait, nil
}

func getBinaryPath(binaryName string) (string, error) {
	binaryPath, err := exec.LookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf("could not find %s: %w", binaryName, err)
	}

	binaryPath, err = filepath.Abs(binaryPath)
	if err != nil {
		return "", fmt.Errorf("could not get absolut path of %s: %w", binaryPath, err)
	}

	return binaryPath, nil
}
