package vm

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"gitlab.com/bootc-org/podman-bootc/pkg/config"

	"github.com/sirupsen/logrus"
	"libvirt.org/go/libvirt"
)

//go:embed domain-template.xml
var domainTemplate string

type BootcVMLinux struct {
	domain            *libvirt.Domain
	libvirtUri        string
	libvirtConnection *libvirt.Connect
	BootcVMCommon
}

func vmName(id string) string {
	return "podman-bootc-" + id[:12]
}

func NewVM(params NewVMParameters) (vm *BootcVMLinux, err error) {
	if params.ImageID == "" {
		return nil, fmt.Errorf("image ID is required")
	}

	if params.LibvirtUri == "" {
		return nil, fmt.Errorf("libvirt URI is required")
	}

	longId, cacheDir, err := GetVMCachePath(params.ImageID, params.User)
	if err != nil {
		return nil, fmt.Errorf("unable to get VM cache path: %w", err)
	}

	lock, err := lockVM(params, cacheDir)
	if err != nil {
		return nil, err
	}

	vm = &BootcVMLinux{
		libvirtUri: params.LibvirtUri,
		BootcVMCommon: BootcVMCommon{
			vmName:        vmName(longId),
			imageID:       longId,
			cacheDir:      cacheDir,
			diskImagePath: filepath.Join(cacheDir, config.DiskImage),
			user:          params.User,
			cacheDirLock:  lock,
		},
	}

	err = vm.loadExistingDomain()
	if err != nil {
		if err := vm.Unlock(); err != nil {
			logrus.Debugf("unlock failed: %v", err)
		}
		return vm, fmt.Errorf("unable to load existing libvirt domain: %w", err)
	}

	return vm, nil
}

func (v *BootcVMLinux) GetConfig() (cfg *BootcVMConfig, err error) {
	cfg, err = v.LoadConfigFile()
	if err != nil {
		return
	}

	cfg.Running, err = v.IsRunning()
	if err != nil {
		return
	}

	return
}

func (v *BootcVMLinux) PrintConsole() (err error) {
	stream, err := v.libvirtConnection.NewStream(libvirt.StreamFlags(0))
	if err != nil {
		return fmt.Errorf("unable to create console stream: %w", err)
	}

	err = v.domain.OpenConsole("serial0", stream, libvirt.DOMAIN_CONSOLE_FORCE)
	if err != nil {
		return fmt.Errorf("unable to open console: %w", err)
	}

	for {
		streamBytes := make([]byte, 8192)
		got, err := stream.Recv(streamBytes)
		if err != nil {
			return fmt.Errorf("unable to receive console output: %w", err)
		}
		if got <= 0 {
			break
		}

		print(string(streamBytes))
	}

	return
}

func (v *BootcVMLinux) Run(params RunVMParameters) (err error) {
	v.sshPort = params.SSHPort
	v.removeVm = params.RemoveVm
	v.background = params.Background
	v.cmd = params.Cmd
	v.hasCloudInit = params.CloudInitData
	v.cloudInitDir = params.CloudInitDir
	v.vmUsername = params.VMUser
	v.sshIdentity = params.SSHIdentity

	if params.NoCredentials {
		v.sshIdentity = ""
		if !v.background {
			fmt.Print("No credentials provided for SSH, using --background by default")
			v.background = true
		}
	}

	if v.domain != nil {
		isRunning, err := v.IsRunning()
		if err != nil {
			return fmt.Errorf("unable to check if VM is running: %w", err)
		}

		if !isRunning {
			logrus.Debugf("Deleting stopped VM %s\n", v.imageID)
			v.Delete()
		} else {
			return errors.New("VM is already running")
		}
	}

	//domain doesn't exist, create it
	logrus.Debugf("Creating VM %s\n", v.imageID)

	domainXML, err := v.parseDomainTemplate()
	if err != nil {
		return fmt.Errorf("unable to parse domain template: %w", err)
	}

	logrus.Debugf("domainXML: %s", domainXML)

	v.domain, err = v.libvirtConnection.DomainDefineXMLFlags(domainXML, libvirt.DOMAIN_DEFINE_VALIDATE)
	if err != nil {
		return fmt.Errorf("unable to define virtual machine domain: %w", err)
	}

	err = v.domain.Create()
	if err != nil {
		return fmt.Errorf("unable to start virtual machine domain: %w", err)
	}

	err = v.waitForVMToBeRunning()
	if err != nil {
		return fmt.Errorf("unable to wait for VM to be running: %w", err)
	}

	return
}

func (v *BootcVMLinux) parseDomainTemplate() (domainXML string, err error) {
	tmpl, err := template.New("domain-template").Parse(domainTemplate)
	if err != nil {
		return "", fmt.Errorf("unable to parse domain template: %w", err)
	}

	var domainXMLBuf bytes.Buffer

	type TemplateParams struct {
		DiskImagePath   string
		Port            string
		PIDFile         string
		SMBios          string
		Name            string
		CloudInitCDRom  string
		CloudInitSMBios string
	}

	templateParams := TemplateParams{
		DiskImagePath: v.diskImagePath,
		Port:          strconv.Itoa(v.sshPort),
		PIDFile:       v.pidFile,
		Name:          v.vmName,
	}

	if v.sshIdentity != "" {
		smbiosCmd, err := v.oemString()
		if err != nil {
			return domainXML, fmt.Errorf("unable to get OEM string: %w", err)
		}

		//this is gross but it's probably better than parsing the XML
		templateParams.SMBios = fmt.Sprintf(`
			<qemu:arg value='-smbios'/>
			<qemu:arg value='%s'/>
		`, smbiosCmd)
	}

	err = v.ParseCloudInit()
	if err != nil {
		return "", fmt.Errorf("unable to set cloud-init: %w", err)
	}

	if v.hasCloudInit {
		templateParams.CloudInitCDRom = fmt.Sprintf(`
			<disk type="file" device="cdrom">
				<driver name="qemu" type="raw"/>
				<source file="%s"></source>
				<target dev="sda" bus="sata"/>
				<readonly/>
			</disk>
		`, v.cloudInitArgs)
	}

	err = tmpl.Execute(&domainXMLBuf, templateParams)
	if err != nil {
		return "", fmt.Errorf("unable to execute domain template: %w", err)
	}

	return domainXMLBuf.String(), nil
}

func (v *BootcVMLinux) waitForVMToBeRunning() error {
	timeout := 60 * time.Second
	elapsed := 0 * time.Second

	for elapsed < timeout {
		state, _, err := v.domain.GetState()

		if err != nil {
			return fmt.Errorf("unable to get VM state: %w", err)
		}

		if state == libvirt.DOMAIN_RUNNING {
			return nil
		}

		time.Sleep(1 * time.Second)
		elapsed += 1 * time.Second
	}

	return fmt.Errorf("VM did not start in %s seconds", timeout)
}

func (v *BootcVMLinux) CloseConnection() {
	v.libvirtConnection.Close()
}

// loadExistingDomain loads the existing domain and it's config, no-op if domain is already loaded
func (v *BootcVMLinux) loadExistingDomain() (err error) {
	//check if domain is already loaded
	if v.domain != nil {
		return
	}

	//look for existing VM
	v.libvirtConnection, err = libvirt.NewConnect(v.libvirtUri)
	if err != nil {
		return
	}

	name := vmName(v.imageID)
	v.domain, err = v.libvirtConnection.LookupDomainByName(name)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_DOMAIN) {
			logrus.Debugf("VM %s not found", name) // allow for domain not found
		} else {
			return
		}
	}

	// if domain exists, load it's config
	if v.domain != nil {
		cfg, err := v.GetConfig()
		if err != nil {
			return fmt.Errorf("unable to load VM config: %w", err)
		}
		v.sshPort = cfg.SshPort
		v.sshIdentity = cfg.SshIdentity
	}

	return nil
}

// Delete the VM definition
func (v *BootcVMLinux) Delete() (err error) {
	err = v.Shutdown()
	if err != nil {
		return fmt.Errorf("unable to shutdown VM: %w", err)
	}

	domainExists, err := v.Exists()
	if err != nil {
		return fmt.Errorf("unable to check if VM exists: %w", err)
	}

	if domainExists {
		err = v.domain.UndefineFlags(libvirt.DOMAIN_UNDEFINE_NVRAM)
		if errors.As(err, &libvirt.Error{Code: libvirt.ERR_INVALID_ARG}) {
			err = v.domain.Undefine()
		}

		if err != nil {
			return fmt.Errorf("unable to undefine VM: %w", err)
		}
	}

	return
}

// Shutdown the VM
func (v *BootcVMLinux) Shutdown() (err error) {
	//check if domain is running and shut it down
	isRunning, err := v.IsRunning()
	if err != nil {
		return fmt.Errorf("unable to check if VM is running: %w", err)
	}

	if isRunning {
		err := v.domain.Destroy()
		if err != nil {
			return fmt.Errorf("unable to destroy VM: %w", err)
		}
	}

	return
}

func (v *BootcVMLinux) Exists() (bool, error) {
	var flags libvirt.ConnectListAllDomainsFlags
	domains, err := v.libvirtConnection.ListAllDomains(flags)
	if err != nil {
		return false, fmt.Errorf("unable to list all domains: %w", err)
	}
	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			return false, err
		}

		if name == v.vmName {
			return true, nil
		}
	}

	return false, nil
}

func (v *BootcVMLinux) IsRunning() (exists bool, err error) {
	if v.domain == nil { // domain hasn't been created yet
		return false, nil
	}

	state, _, err := v.domain.GetState()
	if err != nil {
		return false, fmt.Errorf("unable to get VM state: %w", err)
	}

	if state == libvirt.DOMAIN_RUNNING {
		return true, nil
	} else {
		return false, nil
	}
}

func (v *BootcVMLinux) Unlock() error {
	return v.cacheDirLock.Unlock()
}
