package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

type VmInstallConfig struct {
	Name           string
	ContainerImage string
	SshPubKey      string
	SyslogHost     string
	SyslogPort     uint64
	Vcpu           uint64
	Mem            uint64
	DiskSize       uint64
	vfsdSocket     string
}

const (
	defaultVCPUs    = 2
	defaultMem      = 2048
	defaultDiskSize = 10
	imageMountPoint = "/mnt/cimages"
)

// installCmd represents the hello command
var (
	/*	installCmd = &cobra.Command{
			Use:     "install IMAGE",
			Short:   "install an OS container",
			Long:    `install an OS container`,
			Args:    cobra.ExactArgs(1),
			Example: `osc install --name fedora-base quay.io/centos-bootc/fedora-bootc:eln`,
			Run:     installOSC,
		}
	*/
	keepOnErr = false
	vmInstOpt = VmInstallConfig{}
	vm        VmConfig
)

/*
	func init() {
		RootCmd.AddCommand(installCmd)

		installCmd.Flags().StringVar(&vmInstOpt.Name, "name", "", "VM's name")
		installCmd.Flags().Uint64Var(&vmInstOpt.Vcpu, "vcpu", defaultVCPUs, "Number of virtual CPUs")
		installCmd.Flags().Uint64Var(&vmInstOpt.Mem, "mem", defaultMem, "Memory in MiB")
		installCmd.Flags().Uint64Var(&vmInstOpt.DiskSize, "disk-size", defaultDiskSize, "Disk size in GiB")
		installCmd.Flags().BoolVar(&keepOnErr, "keep-on-error", false, "Do not remove files on failed install")

		// TODO (?)
		// --ks-file 		Add your own kickstart file
		// --ignition   	Path to ignition file
		// --cloud-init 	Path to cloud init config
		// --vm-definition	Path to libvirt xml domain definition
		// --disk-image		Path to disk image (such as osbuild/virt-install output)
	}
*/
func installOSC(_ *cobra.Command, args []string) {
	err := doInstallOSC(args)
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doInstallOSC(args []string) error {
	imageUrl, err := url.Parse(args[0])
	if err != nil {
		return err
	}
	vmInstOpt.ContainerImage = imageUrl.Path

	if vmInstOpt.Name == "" {
		basePath := path.Base(imageUrl.Path)
		vmInstOpt.Name = strings.ReplaceAll(basePath, ":", "-")
	}
	exist, err := isCreated(vmInstOpt.Name)
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("VM '%s' already exist", vm.Name)
	}

	isPodImg, err := isPodmanImage(imageUrl.Path)
	if err != nil {
		return err
	}

	if isPodImg {
		probeVm := NewVMPartial(vmInstOpt.Name)
		imgDir := filepath.Join(probeVm.RunDir(), "image", imageUrl.Path)
		_ = os.MkdirAll(imgDir, os.ModePerm)

		if err := podmanSave(imageUrl.Path, imgDir); err != nil {
			return err
		}

		// virtiofsd mount point (in the guest)
		vmInstOpt.ContainerImage = filepath.Join(imageMountPoint, imageUrl.Path) + " --transport oci"

		socket := filepath.Join(probeVm.RunDir(), "vfsd.sock")
		if err := startVirtiofsd(filepath.Dir(imgDir), socket); err != nil {
			return err
		}
		vmInstOpt.vfsdSocket = socket
	}

	err = doInstall(vmInstOpt)
	if err != nil {
		if !keepOnErr {
			Remove(vmInstOpt.Name)
		}
		return err
	}

	fmt.Println("Installed")
	return nil
}

func doInstall(vmInstOpt VmInstallConfig) error {
	vm = NewVM(vmInstOpt.Name, vmInstOpt.Vcpu, vmInstOpt.Mem, vmInstOpt.DiskSize)

	_ = os.MkdirAll(vm.RunDir(), os.ModePerm)
	_ = os.MkdirAll(vm.ConfigDir(), os.ModePerm)

	if err := newVM(vm); err != nil {
		return err
	}

	// Create ssh keys
	pubSshKey, err := CreateSSHKeys(vm.SshPriKey)
	if err != nil {
		return err
	}
	vmInstOpt.SshPubKey = pubSshKey

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start syslog server
	vmInstOpt.SyslogHost = "10.0.2.2"
	vmInstOpt.SyslogPort = 51451

	// start kickstart file server
	ks, err := NewKickStartFileServer(vmInstOpt)
	if err != nil {
		return err
	}

	ksServerPort, err := ks.Serve(ctx)
	if err != nil {
		return err
	}

	ksUrl := fmt.Sprintf("http://10.0.2.2:%d", ksServerPort)
	if err := installVM(ksUrl, vm, vmInstOpt.vfsdSocket); err != nil {
		return err
	}

	return nil
}

func newVM(vm VmConfig) error {
	vmConfig, err := json.Marshal(vm)
	if err != nil {
		return err
	}

	err = os.WriteFile(vm.ConfigFile(), vmConfig, 0660)
	if err != nil {
		return err
	}

	err = createDiskImage(vm.DiskImage, vm.DiskSize)
	return err
}

func createDiskImage(path string, size uint64) error {
	var args []string
	args = append(args, "create", "-f", "qcow2")
	args = append(args, path)
	diskSizeCmd := fmt.Sprintf("%sG", strconv.FormatUint(size, 10))
	args = append(args, diskSizeCmd)

	cmd := exec.Command("qemu-img", args...)
	err := cmd.Run()
	return err
}

func installVM(ksUrl string, vm VmConfig, vfsdSocket string) error {
	var args []string
	args = append(args, "-nographic")
	mem := strconv.FormatUint(vm.Mem, 10)
	args = append(args, "-object", fmt.Sprintf("memory-backend-file,id=mem,size=%sM,mem-path=/dev/shm,share=on", mem))
	args = append(args, "-machine", "memory-backend=mem,accel=kvm")
	args = append(args, "-cpu", "host", "-nic", "user,model=virtio-net-pci")
	memSizeCmd := fmt.Sprintf("%sM", mem)
	args = append(args, "-m", memSizeCmd)
	args = append(args, "-smp", strconv.FormatUint(vm.Vcpu, 10))

	args = append(args, "-cdrom", IsoImage)
	args = append(args, "-kernel", Kernel)
	args = append(args, "-initrd", Initrd)
	args = append(args, "-pidfile", vm.InstallPidFile())

	driveCmd := fmt.Sprintf("if=virtio,file=%s", vm.DiskImage)
	args = append(args, "-drive", driveCmd)
	appendCmd := fmt.Sprintf("console=ttyS0 inst.ks=%s", ksUrl)

	if vfsdSocket != "" {
		args = append(args, "-chardev", fmt.Sprintf("socket,id=vfsdsock,path=%s", vfsdSocket))
		args = append(args, "-device", "vhost-user-fs-pci,id=vfsd_dev,queue-size=1024,chardev=vfsdsock,tag=host")
		appendCmd = fmt.Sprintf("console=ttyS0 inst.ks=%s systemd.mount-extra=host:%s:virtiofs", ksUrl, imageMountPoint)
	}
	args = append(args, "-append", appendCmd)

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

// KickStart server
const ksFile = "text --non-interactive\n\n" +
	"logging --host={{.SyslogHost}} --port={{.SyslogPort}}\n" +
	"# Basic partitioning\n" +
	"clearpart --all --initlabel --disklabel=gpt\n" +
	"part prepboot  --size=4    --fstype=prepboot\n" +
	"part biosboot  --size=1    --fstype=biosboot\n" +
	"part /boot/efi --size=100  --fstype=efi\n" +
	"part /boot     --size=1000  --fstype=ext4 --label=boot\n" +
	"part / --grow --fstype xfs\n\n" +
	"ostreecontainer --url {{.ContainerImage}} --no-signature-verification\n\n" +
	"firewall --disabled\n" +
	"services --enabled=sshd\n\n" +
	"# Only inject a SSH key for root\n" +
	"rootpw --iscrypted locked\n" +
	"# Add your example SSH key here!\n" +
	"#sshkey --username root \"ssh-ed25519 <key> demo@example.com\"\n" +
	"sshkey --username root \"{{.SshPubKey}}\"\n" +
	"poweroff\n\n" +
	"# Workarounds until https://github.com/rhinstaller/anaconda/pull/5298/ lands\n" +
	"bootloader --location=none --disabled\n" +
	"%post --erroronfail\n" +
	"set -euo pipefail\n" +
	"# Work around anaconda wanting a root password\n" +
	"passwd -l root\n" +
	"rootdevice=$(findmnt -nv -o SOURCE /)\n" +
	"device=$(lsblk -n -o PKNAME ${rootdevice})\n" +
	"/usr/bin/bootupctl backend install --auto --with-static-configs --device /dev/${device} /\n" +
	"%end\n"

type KickStart struct {
	file string
}

func NewKickStartFileServer(cfg VmInstallConfig) (*KickStart, error) {
	tmpl, err := template.New("test").Parse(ksFile)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	err = tmpl.Execute(&output, cfg)
	if err != nil {
		return nil, err
	}
	ks := &KickStart{output.String()}
	return ks, nil
}

func (k *KickStart) Serve(ctx context.Context) (int, error) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _r *http.Request) {
		fmt.Fprintln(w, k.file)
	})
	return httpServe(ctx, handler)
}

// returns port
func httpServe(ctx context.Context, handler http.Handler) (int, error) {
	//listener, err := net.Listen("tcp", "0.0.0.0:0")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return -1, err
	}

	srv := &http.Server{Handler: handler}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	go func() {
		err := srv.Serve(listener)
		if err != nil {
			log.Print("kickstart http file server: ", err)
		}
	}()

	return listener.Addr().(*net.TCPAddr).Port, nil
}

// SSH Key (stolen from podman machine)

// CreateSSHKeys makes a priv and pub ssh key for interacting with a VM.
func CreateSSHKeys(writeLocation string) (string, error) {
	// If the SSH key already exists, hard fail
	if _, err := os.Stat(writeLocation); err == nil {
		return "", fmt.Errorf("SSH key already exists: %s", writeLocation)
	}

	if err := os.MkdirAll(filepath.Dir(writeLocation), 0700); err != nil {
		return "", err
	}

	if err := generatekeys(writeLocation); err != nil {
		return "", err
	}

	b, err := os.ReadFile(writeLocation + ".pub")
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(b), "\n"), nil
}

// generatekeys creates an ed25519 set of keys
func generatekeys(writeLocation string) error {
	var args []string
	args = append(args, "-N", "", "-t", "ed25519", "-f", writeLocation)

	cmd := exec.Command("ssh-keygen", args...)
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	waitErr := cmd.Wait()
	if waitErr == nil {
		return nil
	}

	errMsg, err := io.ReadAll(stdErr)
	if err != nil {
		return fmt.Errorf("key generation failed, unable to read from stderr: %w", waitErr)
	}

	return fmt.Errorf("failed to generate keys: %s: %w", string(errMsg), waitErr)
}

func isPodmanImage(image string) (bool, error) {
	var args []string
	args = append(args, "images", "--format", "json")
	out, err := exec.Command("podman", args...).Output()
	if err != nil {
		return false, err
	}

	var tmp []interface{}
	if err := json.Unmarshal(out, &tmp); err != nil {
		return false, err
	}
	if len(tmp) == 0 {
		return false, nil
	}

	for _, obj := range tmp {
		o := obj.(map[string]interface{})
		id := o["Id"].(string)
		short := id[:12]

		if image == id || image == short {
			return true, nil
		}
	}

	return false, nil
}

func podmanSave(id string, output string) error {
	var args []string
	args = append(args, "save", "--format", "oci-dir", "-o", output, id)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func startVirtiofsd(sharedDir, socketPath string) error {
	var args []string
	args = append(args, "--cache", "always", "--sandbox", "none")
	args = append(args, "--socket-path", socketPath, "--shared-dir", sharedDir)

	cmd := exec.Command("/usr/libexec/virtiofsd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}
