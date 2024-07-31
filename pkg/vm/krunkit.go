package vm

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containers/podman-bootc/pkg/utils"

	"github.com/sirupsen/logrus"
)

const krunkitBinaryName = "krunkit"

const defaultCpus = 4
const defaultMemory = 2048

type krunkitParams struct {
	disk      string
	netSocket string
	oemString string
	pidFile   string
}

type krunkit struct {
	pidFile string
	cmd     *exec.Cmd
}

func newKrunkit(ctx context.Context, binaryPath string, params krunkitParams) *krunkit {
	cmdLine := newKrunkitCmdLine(defaultCpus, defaultMemory)
	cmdLine.addRngDevice()
	cmdLine.addBlockDevice(params.disk)
	cmdLine.addNetworkDevice(params.netSocket)
	cmdLine.addOemString(params.oemString)

	cmdLineSlice := cmdLine.asSlice()
	cmd := exec.CommandContext(ctx, binaryPath, cmdLineSlice...)
	logrus.Debugf("krunkit command-line: %s %s", binaryPath, strings.Join(cmdLineSlice, " "))

	return &krunkit{cmd: cmd, pidFile: params.pidFile}
}

func (k *krunkit) start() error {
	if err := k.cmd.Start(); err != nil {
		return fmt.Errorf("unable to start krunkit: %w", err)
	}

	if err := utils.WritePidFile(k.pidFile, k.cmd.Process.Pid); err != nil {
		if err := k.cmd.Cancel(); err != nil {
			logrus.Debugf("stopping krunkit: %v", err)
		}
		return fmt.Errorf("writing pid file %s: %w", k.pidFile, err)
	}

	return nil
}

func (k *krunkit) wait() error {
	return k.cmd.Wait()
}

type krunkitCmdLine struct {
	cpus      int
	memory    int
	devices   []string
	oemString []string
}

func newKrunkitCmdLine(cpus int, memory int) *krunkitCmdLine {
	return &krunkitCmdLine{cpus: cpus, memory: memory}
}

func (kc *krunkitCmdLine) addDevice(device string) {
	kc.devices = append(kc.devices, device)
}

func (kc *krunkitCmdLine) addRngDevice() {
	kc.addDevice("virtio-rng")
}

func (kc *krunkitCmdLine) addBlockDevice(diskAbsPath string) {
	kc.addDevice(fmt.Sprintf("virtio-blk,path=%s", diskAbsPath))
}

func (kc *krunkitCmdLine) addNetworkDevice(socketAbsPath string) {
	kc.addDevice(fmt.Sprintf("virtio-net,unixSocketPath=%s,mac=5a:94:ef:e4:0c:ee", socketAbsPath))
}

func (kc *krunkitCmdLine) addOemString(oemStr string) {
	kc.oemString = append(kc.oemString, oemStr)
}

func (kc *krunkitCmdLine) asSlice() []string {
	args := []string{}

	args = append(args, "--cpus", strconv.Itoa(kc.cpus))
	args = append(args, "--memory", strconv.Itoa(kc.memory))

	for _, device := range kc.devices {
		args = append(args, "--device", device)
	}

	for _, oemStr := range kc.oemString {
		args = append(args, "--oem-string", oemStr)
	}
	return args
}
