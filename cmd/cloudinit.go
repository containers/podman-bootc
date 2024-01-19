package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func SetCloudInit(id, option string) error {
	if option == "" {
		return errors.New("empty option")
	}

	transport := getTransport(option)
	path := getPath(option)

	if transport == BootcCiDefaultTransport {
		return createCiDataIso(id, path)
	}

	return errors.New("unknown transport")
}

func getTransport(option string) string {
	if strings.Contains(option, ":") {
		return option[:strings.IndexByte(option, ':')]
	}
	return BootcCiDefaultTransport
}

func getPath(option string) string {
	if strings.Contains(option, ":") {
		return option[strings.IndexByte(option, ':'):]
	}
	return option
}

func createCiDataIso(id, inDir string) error {
	vmDir := filepath.Join(CacheDir, id)
	isoOutFile := filepath.Join(vmDir, BootcCiDataIso)

	var args []string
	args = append(args, "-output", isoOutFile)
	args = append(args, "-volid", "cidata", "-joliet", "-rock", "-partition_cyl_align", "on")
	args = append(args, inDir)

	cmd := exec.Command("xorrisofs", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
