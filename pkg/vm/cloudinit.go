package vm

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/utils"
)

func SetCloudInit(id, option string) (int, error) {
	if option == "" {
		return -1, errors.New("empty option")
	}

	transport := getTransport(option)
	path := getPath(option)

	if transport == config.CiDefaultTransport {
		return -1, createCiDataIso(id, path)
	}

	if transport == "imds" {
		port, err := httpServer(path)
		if err != nil {
			return -1, fmt.Errorf("setting up cloud init http server: %w", err)
		}
		return port, nil
	}

	return -1, errors.New("unknown transport")
}

func getTransport(option string) string {
	if strings.Contains(option, ":") {
		return option[:strings.IndexByte(option, ':')]
	}
	return config.CiDefaultTransport
}

func getPath(option string) string {
	if strings.Contains(option, ":") {
		return option[strings.IndexByte(option, ':')+1:]
	}
	return option
}

func createCiDataIso(id, inDir string) error {
	vmDir := filepath.Join(config.CacheDir, id)
	isoOutFile := filepath.Join(vmDir, config.CiDataIso)

	args := []string{"-output", isoOutFile}
	args = append(args, "-volid", "cidata", "-joliet", "-rock", "-partition_cyl_align", "on")
	args = append(args, inDir)

	cmd := exec.Command("xorrisofs", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func httpServer(path string) (int, error) {
	httpPort, err := utils.GetFreeLocalTcpPort()
	if err != nil {
		return -1, err
	}

	fs := http.FileServer(http.Dir(path))
	http.Handle("/", fs)

	go func() {
		err = http.ListenAndServe("127.0.0.1:"+strconv.Itoa(httpPort), nil)
		if err != nil {
			log.Println("Error cloud-init http server: ", err)
		}
	}()
	return httpPort, nil
}
