package vm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"podman-bootc/pkg/config"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type BootcVMParameters struct {
	RemoveVm      bool
	Background    bool
	Directory     string
	User          string
	Name          string
	Cmd           []string
	ImageID       string
	ImageDigest   string
	CloudInitDir  string
	NoCredentials bool
	CloudInitData bool
	SSHIdentity   string
	SSHPort       int
}

type BootcVM interface {
	Run() error
	ForceDelete() error
	Shutdown() error
	Delete() error
	IsRunning() (bool, error)
	WriteConfig() error
	WaitForSSHToBeReady() error
	RunSSH([]string) error
	DeleteFromCache() error
	Exists() (bool, error)
}

type BootcVMCommon struct {
	vmName        string
	directory     string
	diskImagePath string
	user          string
	sshIdentity   string
	sshPort       int
	removeVm      bool
	background    bool
	name          string
	cmd           []string
	pidFile       string
	imageID       string
	imageDigest   string
	noCredentials bool
	hasCloudInit  bool
	cloudInitDir  string
	cloudInitArgs string
}

type BootcVMConfig struct {
	SshPort     int    `json:"SshPort"`
	SshIdentity string `json:"SshPriKey"`
	Repository  string `json:"Image"`
	Tag         string `json:"Tag"`
}

// writeConfig writes the configuration for the VM to the disk
func (v *BootcVMCommon) WriteConfig() error {
	bcConfig := BootcVMConfig{SshPort: v.sshPort, SshIdentity: v.sshIdentity}
	bcConfigMsh, err := json.Marshal(bcConfig)
	if err != nil {
		return fmt.Errorf("marshal config data: %w", err)
	}
	cfgFile := filepath.Join(v.directory, config.CfgFile)
	err = os.WriteFile(cfgFile, bcConfigMsh, 0660)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil

}

func (v *BootcVMCommon) loadConfig() (err error) {
	vmPath, err := v.bootcImagePath(v.imageID)
	if err != nil {
		return
	}

	cfgFile := filepath.Join(vmPath, config.CfgFile)
	fileContent, err := os.ReadFile(cfgFile)
	if err != nil {
		return
	}

	cfg := new(BootcVMConfig)
	if err = json.Unmarshal(fileContent, cfg); err != nil {
		return
	}
	
	v.sshPort = cfg.SshPort
	v.sshIdentity = cfg.SshIdentity
	v.directory = vmPath

	return
}

func (v *BootcVMCommon) bootcImagePath(id string) (string, error) {
	files, err := os.ReadDir(config.CacheDir)
	if err != nil {
		return "", err
	}

	imageId := ""
	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(f.Name(), id) {
			imageId = f.Name()
		}
	}

	if imageId == "" {
		return "", fmt.Errorf("local installation '%s' does not exists", id)
	}

	return filepath.Join(config.CacheDir, imageId), nil
}

func (v *BootcVMCommon) SetUser(user string) error {
	if user == "" {
		return fmt.Errorf("user is required")
	}

	v.user = user
	return nil
}

func (v *BootcVMCommon) WaitForSSHToBeReady() error {
	fmt.Println("Waiting for SSH to be ready")
	timeout := 60 * time.Second
	elapsed := 0 * time.Second

	key, err := os.ReadFile(v.sshIdentity)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %s\n", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %s\n", err)
	}

	config := &ssh.ClientConfig{
		User: v.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         1 * time.Second,
	}

	for elapsed < timeout {
		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", v.sshPort), config)
		if err != nil {
			logrus.Debugf("failed to connect to SSH server: %s\n", err)
			time.Sleep(1 * time.Second)
			elapsed += 1 * time.Second
		} else {
			client.Close()
			return nil
		}
	}

	return fmt.Errorf("SSH did not become ready in %s seconds", timeout)
}

// RunSSH runs a command over ssh or starts an interactive ssh connection if no command is provided
func (v *BootcVMCommon) RunSSH(inputArgs []string) error {
	sshDestination := v.user + "@localhost"
	port := strconv.Itoa(v.sshPort)

	args := []string{"-i", v.sshIdentity, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "StrictHostKeyChecking=no",
		"-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", v.name)
	}

	cmd := exec.Command("ssh", args...)

	logrus.Debugf("Running ssh command: %s", cmd.String())

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Delete removes the VM disk image and the VM configuration from the podman-bootc cache
func (v *BootcVMCommon) DeleteFromCache() error {
	return os.RemoveAll(v.directory)
}

func (b *BootcVMCommon) oemString() (string, error) {
	tmpFilesCmd, err := b.tmpFileInjectSshKeyEnc()
	if err != nil {
		return "", err
	}
	oemString := fmt.Sprintf("type=11,value=io.systemd.credential.binary:tmpfiles.extra=%s", tmpFilesCmd)
	return oemString, nil
}

func (b *BootcVMCommon) tmpFileInjectSshKeyEnc() (string, error) {
	pubKey, err := os.ReadFile(b.sshIdentity + ".pub")
	if err != nil {
		return "", err
	}
	pubKeyEnc := base64.StdEncoding.EncodeToString(pubKey)

	userHomeDir := "/root"
	if b.user != "root" {
		userHomeDir = filepath.Join("/home", b.user)
	}

	tmpFileCmd := fmt.Sprintf("d %[1]s/.ssh 0750 %[2]s %[2]s -\nf+~ %[1]s/.ssh/authorized_keys 700 %[2]s %[2]s - %[3]s", userHomeDir, b.user, pubKeyEnc)

	tmpFileCmdEnc := base64.StdEncoding.EncodeToString([]byte(tmpFileCmd))
	return tmpFileCmdEnc, nil
}
