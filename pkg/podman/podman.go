package podman

import (
	"os"
	"os/exec"
)

// TODO merge with the version in https://github.com/cgwalters/podman/commits/machine-exec/
func PodmanRecurse(args []string) *exec.Cmd {
	c := exec.Command("podman", args...)
	// Default always to using podman machine via the root connection
	c.Env = append(c.Environ(), "CONTAINER_CONNECTION=podman-machine-default-root")
	return c
}

// podmanRecurseRun synchronously runs podman as a subprocess, propagating stdout/stderr
func PodmanRecurseRun(args []string) error {
	c := PodmanRecurse(args)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
