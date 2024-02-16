package cmd

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"podmanbootc/pkg/config"
)

func SetCloudInit(id, option string) (int, error) {
	if option == "" {
		return -1, errors.New("empty option")
	}

	transport := getTransport(option)
	path := getPath(option)

	if transport == BootcCiDefaultTransport {
		return -1, createCiDataIso(id, path)
	}

	if transport == "imds" {
		port, err := httpServer(path)
		if err != nil {
			return -1, err
		}
		return port, nil
	}

	return -1, errors.New("unknown transport")
}

func getTransport(option string) string {
	if strings.Contains(option, ":") {
		return option[:strings.IndexByte(option, ':')]
	}
	return BootcCiDefaultTransport
}

func getPath(option string) string {
	if strings.Contains(option, ":") {
		return option[strings.IndexByte(option, ':')+1:]
	}
	return option
}

func createCiDataIso(id, inDir string) error {
	vmDir := filepath.Join(config.CacheDir, id)
	isoOutFile := filepath.Join(vmDir, config.BootcCiDataIso)

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

func httpServer(path string) (int, error) {
	httpPort, err := getFreeTcpPort()
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
