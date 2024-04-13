package utils

import (
	"errors"
	"os/exec"
)

const (
	PodmanMachineErrorMessage = `
******************************************************************
**** A rootful Podman machine is required to run podman-bootc ****
******************************************************************
`
	// PodmanMachineErrorMessage = "\n**** A rootful Podman machine is required to run podman-bootc ****\n"
)

// SetExitCode set the exit code for exec.ExitError errors, and no
// error is returned
func WithExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), nil
	}
	return 1, err
}
