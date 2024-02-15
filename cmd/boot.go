package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"podmanbootc/pkg/config"
)

type osVmConfig struct {
	Remote          bool
	User            string
	SshIdentity     string
	InjSshIdentity  bool
	GenSshIdentity  bool
	CloudInitDir    string
	KsFile          string
	Interactive     bool
	RemoveVm        bool // Kill the running VM when it exits
	RemoveDiskImage bool // After exit of the VM, remove the disk image
}

var (
	// listCmd represents the hello command
	bootCmd = &cobra.Command{
		Use:          "boot",
		Short:        "Boot OS Containers",
		Long:         "Boot OS Containers",
		Args:         cobra.ExactArgs(1),
		RunE:         boot,
		SilenceUsage: true,
	}

	vmConfig = osVmConfig{}
)

func init() {
	RootCmd.AddCommand(bootCmd)
	bootCmd.Flags().BoolVarP(&vmConfig.Remote, "remote", "r", false, "--remote")
	bootCmd.Flags().StringVarP(&vmConfig.User, "user", "u", "root", "--user <user name> (default: root)")

	// I don't want to deal with cobra quirks right now, let's use multiple options
	bootCmd.Flags().StringVar(&vmConfig.SshIdentity, "ssh-identity", config.DefaultIdentity, "--ssh-identity <identity file>")
	bootCmd.Flags().BoolVar(&vmConfig.InjSshIdentity, "inj-ssh-identity", false, "--inj-ssh-identity")
	bootCmd.Flags().BoolVar(&vmConfig.GenSshIdentity, "gen-ssh-identity", false, "--gen-ssh-identity (implies --inj-ssh-identity)")

	bootCmd.Flags().StringVar(&vmConfig.CloudInitDir, "cloudinit", "", "--cloudinit [[transport:]cloud-init data directory] (transport: cdrom | imds)")

	bootCmd.Flags().BoolVarP(&vmConfig.Interactive, "interactive", "i", false, "-i")
	bootCmd.Flags().BoolVar(&vmConfig.RemoveVm, "rm", false, "Kill the running VM when it exits, requires --interactive")
	//bootCmd.Flags().BoolVar(&vmConfig.RemoveDiskImage, "rmi", false, "After exit of the VM, remove the disk image") // TODO: it requires a monitor process

	// Unsupported yet
	bootCmd.Flags().StringVar(&vmConfig.KsFile, "ks", "", "--ks [kickstart file]") // TODO

}

// TODO merge with the version in https://github.com/cgwalters/podman/commits/machine-exec/
func podmanRecurse(args []string) *exec.Cmd {
	c := exec.Command("podman", args...)
	// Default always to using podman machine via the root connection
	c.Env = append(c.Environ(), "CONTAINER_CONNECTION=podman-machine-default-root")
	return c
}

// podmanRecurseRun synchronously runs podman as a subprocess, propagating stdout/stderr
func podmanRecurseRun(args []string) error {
	c := podmanRecurse(args)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func boot(flags *cobra.Command, args []string) error {

	if vmConfig.GenSshIdentity && flags.Flags().Changed("ssh-identity") {
		return fmt.Errorf("incompatible options: --ssh-identity and --gen-ssh-identity")
	}

	imageName := args[0]

	// Pull the image if not present
	start := time.Now()
	// Run an inspect to see if the image is present, otherwise pull.
	// TODO: Add podman pull --if-not-present or so.
	c := podmanRecurse([]string{"image", "inspect", "-f", "{{.Digest}}", imageName})
	if err := c.Run(); err != nil {
		logrus.Debugf("Inspect failed: %v", err)
		if err := podmanRecurseRun([]string{"pull", imageName}); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}
	elapsed := time.Since(start)
	fmt.Println("getImage elapsed: ", elapsed)

	c = podmanRecurse([]string{"image", "inspect", "-f", "{{.Digest}}", imageName})
	buf := &bytes.Buffer{}
	c.Stdout = buf
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to inspect %s: %w", imageName, err)
	}
	imageDigest := strings.TrimSpace(buf.String())

	// Create VM cache dir; for now we have a single global one, so if
	// you boot a different container image, then any previous disk
	// images are GC'd.
	vmDir := filepath.Join(config.CacheDir)
	if err := os.MkdirAll(vmDir, os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %w", err)
	}

	// install
	start = time.Now()
	err := installImage(vmDir, imageName)
	if err != nil {
		return fmt.Errorf("installImage: %w", err)
	}
	elapsed = time.Since(start)
	fmt.Println("installImage elapsed: ", elapsed)

	// run the new image

	// cloud-init required?
	ciPort := -1 // for http transport
	ciData := flags.Flags().Changed("cloudinit")
	if ciData {
		ciPort, err = SetCloudInit(imageDigest, vmConfig.CloudInitDir)
		if err != nil {
			return fmt.Errorf("setting up cloud init failed: %w", err)
		}
	}

	// Generate ssh credentials
	injectSshKey := vmConfig.InjSshIdentity
	if vmConfig.GenSshIdentity {
		injectSshKey = true
		vmConfig.SshIdentity = filepath.Join(vmDir, BootcSshKeyFile)
		_ = os.Remove(vmConfig.SshIdentity)
		_ = os.Remove(vmConfig.SshIdentity + ".pub")
		if err := generatekeys(vmConfig.SshIdentity); err != nil {
			return fmt.Errorf("ssh generatekeys: %w", err)
		}
	}

	sshPort, err := getFreeTcpPort()
	if err != nil {
		return fmt.Errorf("ssh getFreeTcpPort: %w", err)
	}

	err = runBootcVM(vmDir, sshPort, vmConfig.User, vmConfig.SshIdentity, injectSshKey, ciData, ciPort)
	if err != nil {
		return fmt.Errorf("runBootcVM: %w", err)
	}

	// write down the config file
	bcConfig := BcVmConfig{SshPort: sshPort, SshIdentity: vmConfig.SshIdentity}
	bcConfigMsh, err := json.Marshal(bcConfig)
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	cfgFile := filepath.Join(vmDir, BootcCfgFile)
	err = os.WriteFile(cfgFile, bcConfigMsh, 0660)
	if err != nil {
		return fmt.Errorf("write cfg file: %w", err)
	}

	// Only for interactive
	if vmConfig.Interactive {
		// wait for VM
		//time.Sleep(5 * time.Second) // just for now
		err = waitForVM(vmDir, sshPort)
		if err != nil {
			return fmt.Errorf("waitForVM: %w", err)
		}

		// ssh into it
		cmd := make([]string, 0)
		err = CommonSSH(vmConfig.User, vmConfig.SshIdentity, imageName, sshPort, cmd)
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}

		if vmConfig.RemoveVm {
			// stop the new VM
			//poweroff := []string{"poweroff"}
			//err = CommonSSH("root", DefaultIdentity, name, sshPort, poweroff)
			err = killVM(vmDir)
			if err != nil {
				return fmt.Errorf("poweroff: %w", err)
			}
		}
	}

	return nil
}

func waitForVM(vmDir string, port int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	err = watcher.Add(vmDir)
	if err != nil {
		return err
	}

	vmPidFile := filepath.Join(vmDir, runPidFile)
	for {
		exists, err := fileExists(vmPidFile)
		if err != nil {
			return err
		}

		if exists {
			break
		}

		select {
		case <-watcher.Events:
		case err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("unknown error")
			}
			return err
		}
	}

	for {
		sshReady, err := portIsOpen(port)
		if err != nil {
			return err
		}

		if sshReady {
			return nil
		}
	}
}

func portIsOpen(port int) (bool, error) {
	timeout := time.Second
	conn, _ := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)), timeout)
	if conn != nil {
		defer conn.Close()
		return true, nil
	}
	return false, nil
}

func killVM(vmDir string) error {
	vmPidFile := filepath.Join(vmDir, runPidFile)
	pid, err := readPidFile(vmPidFile)
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(os.Interrupt)
}

func runBootcVM(vmDir string, sshPort int, user, sshIdentity string, injectKey, ciData bool, ciPort int) error {
	var args []string
	args = append(args, "-accel", "kvm", "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", sshPort)
	args = append(args, "-nic", nicCmd)
	//args = append(args, "-nographic")

	vmPidFile := filepath.Join(vmDir, runPidFile)
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(vmDir, BootcDiskImage)
	driveCmd := fmt.Sprintf("if=virtio,format=raw,file=%s", vmDiskImage)
	args = append(args, "-drive", driveCmd)
	if ciData {
		if ciPort != -1 {
			// http cloud init data transport
			// FIXME: this IP address is qemu specific, it should be configurable.
			smbiosCmd := fmt.Sprintf("type=1,serial=ds=nocloud;s=http://10.0.2.2:%d/", ciPort)
			args = append(args, "-smbios", smbiosCmd)
		} else {
			// cdrom cloud init data transport
			ciDataIso := filepath.Join(vmDir, BootcCiDataIso)
			args = append(args, "-cdrom", ciDataIso)
		}
	}

	if injectKey {
		smbiosCmd, err := oemString(user, sshIdentity)
		if err != nil {
			return err
		}

		args = append(args, "-smbios", smbiosCmd)
	}

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func installImage(vmdir, imageName string) error {
	// Create a raw disk image
	imgFileName := filepath.Join(vmdir, BootcDiskImage)
	imgFile, err := os.Create(imgFileName)
	if err != nil {
		return err
	}
	if err := imgFile.Truncate(10e+9); err != nil {
		return err
	}

	// https://github.com/containers/bootc/blob/main/docs/install.md#using-bootc-install-to-disk---via-loopback
	volumeBind := fmt.Sprintf("%s:/output", vmdir)
	installArgsForPodman := []string{"run", "--rm", "--privileged", "--pid=host", "-v", volumeBind, "--security-opt", "label=type:unconfined_t"}
	if val, ok := os.LookupEnv("PODMAN_BOOTC_INST_ARGS"); ok {
		parts := strings.Split(val, " ")
		installArgsForPodman = append(installArgsForPodman, parts...)
	}
	installArgsForPodman = append(installArgsForPodman, imageName)
	installArgsForBootc := []string{"bootc", "install", "to-disk", "--via-loopback", "--generic-image", "--skip-fetch-check", "/output/" + BootcDiskImage}
	if err := podmanRecurseRun(append(installArgsForPodman, installArgsForBootc...)); err != nil {
		return fmt.Errorf("failed to generate disk image via bootc install to-disk --via-loopback")
	}

	return nil
}

func pullImage(containerImage string, remote bool) error {
	var args []string
	if remote {
		args = append(args, "-r")
	}

	args = append(args, "pull", containerImage)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func saveImage(id string) error {
	var args []string
	output := filepath.Join(config.CacheDir, id, BootcOciArchive)
	args = append(args, "save", "--format", "oci-archive", "-o", output, id)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
