package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type osVmConfig struct {
	Name          string
	CloudInitFile string
	KsFile        string
	//ContainerImage string
	//SshPubKey      string
}

var (
	// listCmd represents the hello command
	bootCmd = &cobra.Command{
		Use:   "boot",
		Short: "Boot OS Containers",
		Long:  "Boot OS Containers",
		Run:   boot,
	}

	vmConfig = osVmConfig{}
)

func init() {
	RootCmd.AddCommand(bootCmd)
	//installCmd.Flags().StringVar(&vmConfig.Name, "name", "", "OS container name")
	//installCmd.Flags().StringVar(&vmConfig.CloudInitFile, "cloudinit", "", "[unimplemented]")
	//installCmd.Flags().StringVar(&vmConfig.KsFile, "ks", "", "[unimplemented]")
}

func boot(_ *cobra.Command, args []string) {

	// Pull the image if not present
	start := time.Now()
	id, name, err := getImage(args[0])
	if err != nil {
		fmt.Println("Error getImage: ", err)
		return
	}
	elapsed := time.Since(start)
	fmt.Println("getImage elapsed: ", elapsed)

	// Create VM cache dir
	vmDir := filepath.Join(CacheDir, id)
	if err := os.MkdirAll(vmDir, os.ModePerm); err != nil {
		fmt.Println("Error MkdirAll: ", err)
		return
	}

	// load the bootc image into the podman default machine
	// (only required on linux)
	start = time.Now()
	err = loadImageToDefaultMachine(id, name)
	if err != nil {
		fmt.Println("Error loadImageToDefaultMachine: ", err)
		return
	}
	elapsed = time.Since(start)
	fmt.Println("loadImageToDefaultMachine elapsed: ", elapsed)

	// install
	start = time.Now()
	err = installImage(id)
	if err != nil {
		fmt.Println("Error installImage: ", err)
		return
	}
	elapsed = time.Since(start)
	fmt.Println("installImage elapsed: ", elapsed)

	// run the new image
	sshPort, err := getFreeTcpPort()
	if err != nil {
		fmt.Println("Error ssh getFreeTcpPort: ", err)
		return
	}

	err = runBootcVM(id, sshPort)
	if err != nil {
		fmt.Println("Error runBootcVM: ", err)
		return
	}

	// wait for VM
	//time.Sleep(5 * time.Second) // just for now
	err = waitForVM(id, sshPort)
	if err != nil {
		fmt.Println("Error waitForVM: ", err)
		return
	}

	// ssh into it
	cmd := make([]string, 0)
	err = CommonSSH("root", DefaultIdentity, name, sshPort, cmd)
	if err != nil {
		fmt.Println("Error ssh: ", err)
		return
	}

	// stop the new VM
	//poweroff := []string{"poweroff"}
	//err = CommonSSH("root", DefaultIdentity, name, sshPort, poweroff)
	err = killVM(id)
	if err != nil {
		fmt.Println("Error poweroff: ", err)
		return
	}
}
func waitForVM(id string, port int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Join(CacheDir, id))
	if err != nil {
		return err
	}

	vmPidFile := filepath.Join(CacheDir, id, runPidFile)
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

func killVM(id string) error {
	vmPidFile := filepath.Join(CacheDir, id, runPidFile)
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

func runBootcVM(id string, sshPort int) error {
	var args []string
	args = append(args, "-accel", "kvm", "-cpu", "host")
	args = append(args, "-m", "2G")
	args = append(args, "-smp", "2")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", sshPort)
	args = append(args, "-nic", nicCmd)
	//args = append(args, "-nographic")

	vmPidFile := filepath.Join(CacheDir, id, runPidFile)
	args = append(args, "-pidfile", vmPidFile)

	vmDiskImage := filepath.Join(CacheDir, id, bootcDiskImage)
	driveCmd := fmt.Sprintf("if=virtio,format=raw,file=%s", vmDiskImage)
	args = append(args, "-drive", driveCmd)

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func loadImageToDefaultMachine(id, name string) error {
	// Save the image to the cache
	err := saveImage(id)
	if err != nil {
		return err
	}

	// Mount the cache directory
	cmd := []string{"mount", "-t", "virtiofs", "osc-cache", "/mnt"}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}
	// Load the image to the podman machine VM
	// (this step is unnecessary in macos or using podman machine in linux, but my podman is too old)
	//podman load -i /mnt/55953d3d5ec33b2e636b044f21f9d1255fbd0b14340c75f4480135349eea908f.tar
	ociImgFileName := filepath.Join("/mnt", id, bootcOciArchive)
	cmd = []string{"podman", "load", "-i", ociImgFileName}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	//podman tag 55953d3d5ec33b2e636b044f21f9d1255fbd0b14340c75f4480135349eea908f quay.io/centos-bootc/fedora-bootc:eln
	cmd = []string{"podman", "tag", id, name}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}
	return nil
}

func installImage(id string) error {
	// Create a raw disk image
	imgFileName := filepath.Join(CacheDir, id, bootcDiskImage)
	imgFile, err := os.Create(imgFileName)
	if err != nil {
		return err
	}
	// just ~5GB
	if err := imgFile.Truncate(5e+9); err != nil {
		return err
	}

	// Installing

	// We assume this will be /dev/loop0
	//losetup --show -P -f /mnt/55953d3d5ec33b2e636b044f21f9d1255fbd0b14340c75f4480135349eea908f.img
	diskImg := filepath.Join("/mnt", id, bootcDiskImage)
	cmd := []string{"losetup", "--show", "-P", "-f", diskImg}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	cmd = []string{"losetup"}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}
	cmd = []string{"podman", "images"}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	//podman run -it --rm --privileged --pid=host --security-opt label=type:unconfined_t 55953d3d5ec33b2e636b044f21f9d1255fbd0b14340c75f4480135349eea908f \
	// bootc install to-disk --wipe --target-no-signature-verification --generic-image --skip-fetch-check  /dev/loop0
	podmanCmd := []string{"podman", "run", "--rm", "--privileged", "--pid=host", "--security-opt", "label=type:unconfined_t", id}
	bootcCmd := []string{"bootc", "install", "to-disk", "--wipe", "--target-no-signature-verification", "--generic-image", "--skip-fetch-check", "/dev/loop0"}
	cmd = append(podmanCmd, bootcCmd...)
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	//losetup -d /dev/loop0
	cmd = []string{"losetup", "-d", "/dev/loop0"}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	//podman image rm 55953d3d5ec33b2e636b044f21f9d1255fbd0b14340c75f4480135349eea908f
	cmd = []string{"podman", "image", "rm", id}
	if err := runOnDefaultMachine(cmd); err != nil {
		return err
	}

	return nil
}

func runOnDefaultMachine(cmd []string) error {
	return CommonSSH("root", MachineIdentity, "default machine", 2222, cmd)
}

func getImage(containerImage string) (string, string, error) {
	// Get the podman image ID
	id, err := getImageId(containerImage)
	if err != nil {
		return "", "", err
	}

	// let's try again adding a tag
	if id == "" {
		// Add "latest" tag if missing
		if !strings.Contains(containerImage, ":") {
			containerImage = containerImage + ":latest"
		}
		id, err = getImageId(containerImage)
		if err != nil {
			return "", "", err
		}
	}

	// Pull the image if it's not present
	if id == "" {
		err := pullImage(containerImage)
		if err != nil {
			return "", "", err
		}
		id, err = getImageId(containerImage)
		if err != nil {
			return "", "", err
		}
	}

	return id, containerImage, nil
}

func getImageId(image string) (string, error) {
	var args []string
	args = append(args, "images", "--format", "json")
	out, err := exec.Command("podman", args...).Output()
	if err != nil {
		return "", err
	}

	var tmp []interface{}
	if err := json.Unmarshal(out, &tmp); err != nil {
		return "", err
	}
	if len(tmp) == 0 {
		return "", nil
	}

	for _, obj := range tmp {
		o := obj.(map[string]interface{})
		id := o["Id"].(string)
		short := id[:12]

		if image == id || image == short {
			return id, nil
		}

		for _, name := range o["Names"].([]interface{}) {
			if image == name {
				return id, nil
			}
		}
	}

	return "", nil
}

func pullImage(containerImage string) error {
	var args []string
	args = append(args, "pull", containerImage)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func saveImage(id string) error {
	var args []string
	output := filepath.Join(CacheDir, id, bootcOciArchive)
	args = append(args, "save", "--format", "oci-archive", "-o", output, id)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
