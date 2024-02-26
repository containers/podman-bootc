package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"podman-bootc/pkg/config"
	"podman-bootc/pkg/disk"
	"podman-bootc/pkg/podman"
	"podman-bootc/pkg/ssh"
	"podman-bootc/pkg/utils"
	"podman-bootc/pkg/vm"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

type osVmConfig struct {
	Remote          bool
	User            string
	CloudInitDir    string
	KsFile          string
	Background      bool
	RemoveVm        bool // Kill the running VM when it exits
	RemoveDiskImage bool // After exit of the VM, remove the disk image
}

var (
	// listCmd represents the hello command
	runCmd = &cobra.Command{
		Use:          "run",
		Short:        "Run a bootc container as a VM",
		Long:         "Run a bootc container as a VM",
		Args:         cobra.ExactArgs(1),
		RunE:         boot,
		SilenceUsage: true,
	}

	vmConfig = osVmConfig{}
)

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&vmConfig.Remote, "remote", "r", false, "--remote")
	runCmd.Flags().StringVarP(&vmConfig.User, "user", "u", "root", "--user <user name> (default: root)")

	runCmd.Flags().StringVar(&vmConfig.CloudInitDir, "cloudinit", "", "--cloudinit [[transport:]cloud-init data directory] (transport: cdrom | imds)")

	runCmd.Flags().BoolVarP(&vmConfig.Background, "background", "B", false, "Do not spawn SSH, run in background")
	runCmd.Flags().BoolVar(&vmConfig.RemoveVm, "rm", false, "Kill the running VM when it exits, requires --interactive")

}

func boot(flags *cobra.Command, args []string) error {
	imageName := args[0]

	imageDigest, err := podman.GetImage(imageName)
	if err != nil {
		return err
	}

	// Create VM cache dir; for now we have a single global one, so if
	// you boot a different container image, then any previous disk
	// images are GC'd.
	vmDir := filepath.Join(config.CacheDir)
	if err := os.MkdirAll(vmDir, os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %w", err)
	}

	// install
	start := time.Now()
	if err := disk.GetOrInstallImage(vmDir, imageName, imageDigest); err != nil {
		return fmt.Errorf("installImage: %w", err)
	}
	elapsed := time.Since(start)
	fmt.Println("installImage elapsed: ", elapsed)

	// run the new image

	privkey, pubkey, err := podman.MachineSSHKey()
	if err != nil {
		return fmt.Errorf("getting podman ssh")
	}

	sshPort, err := utils.GetFreeLocalTcpPort()
	if err != nil {
		return fmt.Errorf("ssh getFreeTcpPort: %w", err)
	}

	// cloud-init required?
	ciPort := -1 // for http transport
	ciData := flags.Flags().Changed("cloudinit")
	if ciData {
		ciPort, err = vm.SetCloudInit(imageDigest, vmConfig.CloudInitDir)
		if err != nil {
			return fmt.Errorf("setting up cloud init failed: %w", err)
		}
	}

	err = vm.RunVM(vmDir, sshPort, vmConfig.User, pubkey, ciData, ciPort)
	if err != nil {
		return fmt.Errorf("runBootcVM: %w", err)
	}

	// write down the config file
	bcConfig := config.BcVmConfig{SshPort: sshPort, SshIdentity: privkey}
	bcConfigMsh, err := json.Marshal(bcConfig)
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	cfgFile := filepath.Join(vmDir, config.CfgFile)
	err = os.WriteFile(cfgFile, bcConfigMsh, 0660)
	if err != nil {
		return fmt.Errorf("write cfg file: %w", err)
	}

	if !vmConfig.Background {
		// wait for VM
		//time.Sleep(5 * time.Second) // just for now
		err = waitForVM(vmDir, sshPort)
		if err != nil {
			return fmt.Errorf("waitForVM: %w", err)
		}

		// ssh into it
		cmd := make([]string, 0)
		err = ssh.CommonSSH(vmConfig.User, privkey, imageName, sshPort, cmd)
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

	vmPidFile := filepath.Join(vmDir, config.RunPidFile)
	for {
		exists, err := utils.FileExists(vmPidFile)
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
	vmPidFile := filepath.Join(vmDir, config.RunPidFile)
	pid, err := utils.ReadPidFile(vmPidFile)
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(os.Interrupt)
}
