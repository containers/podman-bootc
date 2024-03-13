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

func (b BootcVMCommon) SetCloudInit() (err error) {
	// cloud-init required?
	b.ciPort = -1 // for http transport
	if b.ciData {
		if b.cloudInitDir == "" {
			return errors.New("empty cloud init directory")
		}

		transport := b.getTransport()
		path := b.getPath()

		if transport == config.CiDefaultTransport {
			return b.createCiDataIso(path)
		}

		if transport == "imds" {
			b.ciPort, err = b.httpServer(path)
			if err != nil {
				return fmt.Errorf("setting up cloud init http server: %w", err)
			}
			return nil
		}

		return errors.New("unknown transport")
	}
	return nil
}

func (b BootcVMCommon) getTransport() string {
	if strings.Contains(b.cloudInitDir, ":") {
		return b.cloudInitDir[:strings.IndexByte(b.cloudInitDir, ':')]
	}
	return config.CiDefaultTransport
}

func (b BootcVMCommon) getPath() string {
	if strings.Contains(b.cloudInitDir, ":") {
		return b.cloudInitDir[strings.IndexByte(b.cloudInitDir, ':')+1:]
	}
	return b.cloudInitDir
}

func (b BootcVMCommon) createCiDataIso(inDir string) error {
	vmDir := filepath.Join(config.CacheDir, b.imageDigest)
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

func (b BootcVMCommon) httpServer(path string) (int, error) {
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
