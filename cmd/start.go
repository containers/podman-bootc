package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"

	"bootc/pkg/config"
)

/*
	var startCmd = &cobra.Command{
		Use:     "start NAME",
		Short:   "Start an existing OS Container machine",
		Long:    "Start an existing OS Container machine",
		Args:    cobra.ExactArgs(1),
		Example: `osc start fedora-base`,
		Run:     startVm,
	}

	func init() {
		RootCmd.AddCommand(startCmd)
	}
*/
func startVm(_ *cobra.Command, args []string) {
	err := doStartVm(args[0])
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doStartVm(name string) error {
	vm, err := LoadVmFromDisk(name)
	if err != nil {
		return err
	}

	if vm.Status() != Stopped {
		return fmt.Errorf("cannot start %s' (state: %s)", name, vm.Status())
	}

	sshPort, err := getFreeTcpPort()
	if err != nil {
		return err
	}

	vncPort, err := countRunningVms()
	if err != nil {
		return err
	}

	runCfg := RunVmConfig{
		SshPort: uint64(sshPort),
		VncPort: uint64(5900 + vncPort),
	}
	err = newRunningVM(vm, runCfg)
	if err != nil {
		return err
	}

	return runQemu(vm, sshPort, vncPort)
}

func newRunningVM(vm *VmConfig, runCfg RunVmConfig) error {
	vmConfig, err := json.Marshal(runCfg)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(vm.RunDir(), os.ModePerm); err != nil {
		return err
	}

	err = os.WriteFile(vm.RunConfigFile(), vmConfig, 0660)
	if err != nil {
		return err
	}
	return nil
}

func runQemu(vm *VmConfig, sshPort, vncPort int) error {
	var args []string
	args = append(args, "-accel", "kvm", "-cpu", "host")
	nicCmd := fmt.Sprintf("user,model=virtio-net-pci,hostfwd=tcp::%d-:22", sshPort)
	args = append(args, "-nic", nicCmd)
	args = append(args, "-pidfile", vm.RunPidFile)

	memSizeCmd := fmt.Sprintf("%sM", strconv.FormatUint(vm.Mem, 10))
	args = append(args, "-m", memSizeCmd)

	args = append(args, "-smp", strconv.FormatUint(vm.Vcpu, 10))

	driveCmd := fmt.Sprintf("if=virtio,file=%s", vm.DiskImage)
	args = append(args, "-drive", driveCmd)
	args = append(args, "-vnc", fmt.Sprintf(":%d", vncPort))

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}

	return nil
}

func getFreeTcpPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return -1, err
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

func countRunningVms() (int, error) {
	files, err := os.ReadDir(config.RunDir)
	if err != nil {
		return -1, err
	}

	count := 0
	for _, f := range files {
		if f.IsDir() {
			probeVm := NewVMPartial(f.Name())
			exist, err := fileExist(probeVm.RunPidFile)
			if err != nil {
				return -1, err
			}
			if exist {
				count += 1
			}
		}
	}
	return count, nil
}
