package utils

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

func ReadPidFile(pidFile string) (int, error) {
	if _, err := os.Stat(pidFile); err != nil {
		return -1, err
	}

	fileContent, err := os.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}
	pidStr := string(bytes.Trim(fileContent, "\n"))
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		return -1, err
	}
	return int(pid), nil
}

func WritePidFile(pidFile string, pid int) error {
	if pid < 1 {
		// We might be running as PID 1 when running docker-in-docker,
		// but 0 or negative PIDs are not acceptable.
		return fmt.Errorf("invalid negative PID %d", pid)
	}
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644)
}

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	exists := false

	if err == nil {
		exists = true
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return exists, err
}

// WaitForFileWithBackoffs attempts to discover a file in maxBackoffs attempts
func WaitForFileWithBackoffs(maxBackoffs int, backoff time.Duration, path string) error {
	backoffWait := backoff
	for i := 0; i < maxBackoffs; i++ {
		e, _ := FileExists(path)
		if e {
			return nil
		}
		time.Sleep(backoffWait)
		backoffWait *= 2
	}
	return fmt.Errorf("unable to find file at %q", path)
}
