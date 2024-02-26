package utils

import (
	"bytes"
	"errors"
	"os"
	"strconv"
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
