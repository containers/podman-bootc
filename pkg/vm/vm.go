package vm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"podman-bootc/pkg/bootc"
	"podman-bootc/pkg/config"
	"podman-bootc/pkg/user"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// getVMCachePath returns the path to the VM cache directory
func getVMCachePath(imageId string, user user.User) (path string, err error) {
	files, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return "", err
	}

	fullImageId := ""
	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(f.Name(), imageId) {
			fullImageId = f.Name()
		}
	}

	if fullImageId == "" {
		return "", fmt.Errorf("local installation '%s' does not exists", imageId)
	}

	return filepath.Join(user.CacheDir(), fullImageId), nil
}

type NewVMParameters struct {
	ImageID    string
	User       user.User //user who is running the podman bootc command
	LibvirtUri string    //linux only
}

type RunVMParameters struct {
	VMUser        string //user to use when connecting to the VM
	CloudInitDir  string
	NoCredentials bool
	CloudInitData bool
	SSHIdentity   string
	SSHPort       int
	Cmd           []string
	RemoveVm      bool
	Background    bool
}

type BootcVM interface {
	Run(RunVMParameters) error
	ForceDelete() error
	Shutdown() error
	Delete() error
	IsRunning() (bool, error)
	WriteConfig(bootc.BootcDisk) error
	WaitForSSHToBeReady() error
	RunSSH([]string) error
	DeleteFromCache() error
	Exists() (bool, error)
	GetConfig() (*BootcVMConfig, error)
	CloseConnection()
	PrintConsole() (error)
}

type BootcVMCommon struct {
	vmName        string
	cacheDir      string
	diskImagePath string
	vmUsername    string
	user          user.User
	sshIdentity   string
	sshPort       int
	removeVm      bool
	background    bool
	cmd           []string
	pidFile       string
	imageID       string
	imageDigest   string
	noCredentials bool
	hasCloudInit  bool
	cloudInitDir  string
	cloudInitArgs string
	bootcDisk     bootc.BootcDisk
}

type BootcVMConfig struct {
	Id          string `json:"Id,omitempty"`
	SshPort     int    `json:"SshPort"`
	SshIdentity string `json:"SshPriKey"`
	RepoTag     string `json:"Repository"`
	Created     string `json:"Created,omitempty"`
	DiskSize    string `json:"DiskSize,omitempty"`
	Running     bool   `json:"Running,omitempty"`
}

// writeConfig writes the configuration for the VM to the disk
func (v *BootcVMCommon) WriteConfig(bootcDisk bootc.BootcDisk) error {
	bcConfig := BootcVMConfig{
		Id:          v.imageID[0:12],
		SshPort:     v.sshPort,
		SshIdentity: v.sshIdentity,
		RepoTag:     bootcDisk.GetRepoTag(),
		Created:     bootcDisk.GetCreatedAt().Format(time.RFC3339),
		DiskSize:    strconv.Itoa(bootcDisk.GetSize()),
	}

	bcConfigMsh, err := json.Marshal(bcConfig)
	if err != nil {
		return fmt.Errorf("marshal config data: %w", err)
	}
	cfgFile := filepath.Join(v.cacheDir, config.CfgFile)
	err = os.WriteFile(cfgFile, bcConfigMsh, 0660)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil

}

func (v *BootcVMCommon) LoadConfigFile() (cfg *BootcVMConfig, err error) {
	cfgFile := filepath.Join(v.cacheDir, config.CfgFile)
	fileContent, err := os.ReadFile(cfgFile)
	if err != nil {
		return
	}

	cfg = new(BootcVMConfig)
	if err = json.Unmarshal(fileContent, cfg); err != nil {
		return
	}

	//format the config values for display
	createdTime, err := time.Parse(time.RFC3339, cfg.Created)
	if err != nil {
		return nil, fmt.Errorf("error parsing created time: %w", err)
	}
	cfg.Created = units.HumanDuration(time.Since(createdTime)) + " ago"

	diskSizeFloat, err := strconv.ParseFloat(cfg.DiskSize, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing disk size: %w", err)
	}
	cfg.DiskSize = units.HumanSizeWithPrecision(diskSizeFloat, 3)

	return
}

func (v *BootcVMCommon) SetUser(user string) error {
	if user == "" {
		return fmt.Errorf("user is required")
	}

	v.vmUsername = user
	return nil
}

func (v *BootcVMCommon) WaitForSSHToBeReady() error {
	timeout := 1 * time.Minute
	elapsed := 0 * time.Millisecond
	interval := 500 * time.Millisecond

	key, err := os.ReadFile(v.sshIdentity)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %s\n", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %s\n", err)
	}

	config := &ssh.ClientConfig{
		User: v.vmUsername,
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
			time.Sleep(interval)
			elapsed += interval
		} else {
			client.Close()
			return nil
		}
	}

	return fmt.Errorf("SSH did not become ready in %s seconds", timeout)
}

// RunSSH runs a command over ssh or starts an interactive ssh connection if no command is provided
func (v *BootcVMCommon) RunSSH(inputArgs []string) error {
	cfg, err := v.LoadConfigFile()
	if err != nil {
		return fmt.Errorf("failed to load VM config: %w", err)
	}

	v.sshPort = cfg.SshPort
	v.sshIdentity = cfg.SshIdentity

	sshDestination := v.vmUsername + "@localhost"
	port := strconv.Itoa(v.sshPort)

	args := []string{"-i", v.sshIdentity, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "StrictHostKeyChecking=no",
		"-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", v.imageID)
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
	return os.RemoveAll(v.cacheDir)
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
	if b.vmUsername != "root" {
		userHomeDir = filepath.Join("/home", b.vmUsername)
	}

	tmpFileCmd := fmt.Sprintf("d %[1]s/.ssh 0750 %[2]s %[2]s -\nf+~ %[1]s/.ssh/authorized_keys 700 %[2]s %[2]s - %[3]s", userHomeDir, b.vmUsername, pubKeyEnc)

	tmpFileCmdEnc := base64.StdEncoding.EncodeToString([]byte(tmpFileCmd))
	return tmpFileCmdEnc, nil
}
