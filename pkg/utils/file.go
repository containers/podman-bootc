package utils

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.com/bootc-org/podman-bootc/pkg/user"
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

// FullImageIdFromPartial returns the full image ID given a partial image ID and the user.
// If the partial image ID is already a full image ID, it is returned as is.
func FullImageIdFromPartial(partialId string, user user.User) (fullImageId string, err error) {
	if len(partialId) == 64 {
		logrus.Debugf("Partial image ID '%s' is already a full image ID", partialId)
		return partialId, nil
	}

	files, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if f.IsDir() && len(f.Name()) == 64 && strings.HasPrefix(f.Name(), partialId) {
			fullImageId = f.Name()
		}
	}

	if fullImageId == "" {
		return "", fmt.Errorf("local installation '%s' does not exists", partialId)
	}

	return fullImageId, nil
}
