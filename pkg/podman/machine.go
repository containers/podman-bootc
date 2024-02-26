package podman

import (
	"bytes"
	"os/exec"
	"strings"
)

func IsDefaultMachineRunning() bool {
	c := exec.Command("podman", []string{"machine", "inspect", "--format", "{{.State}}"}...)
	buf := &bytes.Buffer{}
	c.Stdout = buf
	if err := c.Run(); err != nil {
		return false
	}

	state := strings.TrimSpace(buf.String())

	// On MacOS, podman machine sometimes stays in the 'starting' state
	return state == "running" || state == "starting"
}
