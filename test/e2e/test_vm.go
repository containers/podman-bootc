package e2e

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type TestVM struct {
	StdIn    io.WriteCloser
	StdOut   []string
	IsBooted bool
	Id       string
}

func (w *TestVM) SetId(id string) {
	w.Id = id
}

func (w *TestVM) GetId() string {
	return w.Id
}

func (w *TestVM) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "Connecting to vm") {
		w.IsBooted = true
	}
	print(string(p))
	w.StdOut = append(w.StdOut, string(p))
	return len(p), nil
}

func (w *TestVM) WaitForBoot() (err error) {
	timeout := 5 * time.Minute
	interval := 1 * time.Second
	for {
		if w.IsBooted {
			break
		}
		time.Sleep(interval)
		timeout -= interval
		if timeout <= 0 {
			return fmt.Errorf("VM did not boot within timeout")
		}
	}

	return
}

// SendCommand sends a command to the VM's stdin and waits for the output
func (w *TestVM) SendCommand(cmd string, output string) (err error) {
	w.StdIn.Write([]byte(cmd + "\n"))

	timeout := 2 * time.Minute
	interval := 1 * time.Second
	for {
		if strings.Index(w.StdOut[len(w.StdOut)-1], output) == 0 {
			break
		}
		time.Sleep(interval)
		timeout -= interval
		if timeout <= 0 {
			return fmt.Errorf("VM did not output expected string within timeout")
		}
	}

	return
}
