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

func (b *BootcVMCommon) ParseCloudInit() (err error) {
	// cloud-init required?
	ciPort := -1 // for http transport
	if b.hasCloudInit {
		if b.cloudInitDir == "" {
			return errors.New("empty cloud init directory")
		}

		transport := b.getTransport()
		path := b.getPath()

		if transport == config.CiDefaultTransport {
			err = b.createCiDataIso(path)
			if err != nil {
				return fmt.Errorf("creating cloud-init iso: %w", err)
			}
		} else if transport == "imds" {
			ciPort, err = b.httpServer(path)
			if err != nil {
				return fmt.Errorf("setting up cloud init http server: %w", err)
			}
		} else {
			return errors.New("unknown cloudinit transport")
		}

		if ciPort != -1 {
			// http cloud init data transport
			// FIXME: this IP address is qemu specific, it should be configurable.
			smbiosCmd := fmt.Sprintf("type=1,serial=ds=nocloud;s=http://10.0.2.2:%d/", ciPort)
			// args = append(args, "-smbios", smbiosCmd)
			b.cloudInitType = "smbios"
			b.cloudInitArgs = smbiosCmd
		} else {
			// cdrom cloud init data transport
			ciDataIso := filepath.Join(b.directory, config.CiDataIso)
			// args = append(args, "-cdrom", ciDataIso)
			b.cloudInitType = "cdrom"
			b.cloudInitArgs = ciDataIso
		}
	}

	return nil
}

func (b *BootcVMCommon) getTransport() string {
	if strings.Contains(b.cloudInitDir, ":") {
		return b.cloudInitDir[:strings.IndexByte(b.cloudInitDir, ':')]
	}
	return config.CiDefaultTransport
}

func (b *BootcVMCommon) getPath() string {
	if strings.Contains(b.cloudInitDir, ":") {
		return b.cloudInitDir[strings.IndexByte(b.cloudInitDir, ':')+1:]
	}
	return b.cloudInitDir
}

func (b *BootcVMCommon) createCiDataIso(inDir string) error {
	isoOutFile := filepath.Join(b.directory, config.CiDataIso)

	args := []string{"-output", isoOutFile}
	args = append(args, "-volid", "cidata", "-joliet", "-rock", "-partition_cyl_align", "on")
	args = append(args, inDir)

	cmd := exec.Command("xorrisofs", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (b *BootcVMCommon) httpServer(path string) (int, error) {
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
