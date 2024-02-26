package utils

import (
	"os"

	"podman-bootc/pkg/config"
)

func InitOSCDirs() error {
	if err := os.MkdirAll(config.ConfigDir, os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(config.CacheDir, os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(config.RunDir(), os.ModePerm); err != nil {
		return err
	}

	return nil
}
