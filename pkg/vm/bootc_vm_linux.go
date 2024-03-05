package vm

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"podman-bootc/pkg/config"
	"strconv"
	"text/template"
	"time"

	"libvirt.org/go/libvirt"
)

//go:embed domain-template.xml
var domainTemplate string

type BootcVMLinux struct {
	domain *libvirt.Domain
	BootcVMCommon
}

func vmName(id string) string {
	return "podman-bootc-" + id[:12]
}

func NewBootcVMLinuxById(id string) (vm BootcVMLinux, err error) {
	//find the domain by id
	conn, err := libvirt.NewConnect("qemu:///session")
	if err != nil {
		return
	}
	defer conn.Close()

	domain, err := conn.LookupDomainByName(vmName(id))
	if err != nil {
		return
	}

	return BootcVMLinux{
		domain: domain,
	}, nil
}

func NewBootcVMLinux(params BootcVMParameters) (BootcVMLinux, error) {
	return BootcVMLinux{
		BootcVMCommon: BootcVMCommon{
			user:          params.User,
			directory:     params.Directory,
			diskImagePath: filepath.Join(params.Directory, config.DiskImage),
			sshIdentity:   params.SSHIdentity,
			sshPort:       params.SSHPort,
			removeVm:      params.RemoveVm,
			background:    params.Background,
			name:          params.Name,
			cmd:           params.Cmd,
			pidFile:       filepath.Join(params.Directory, config.RunPidFile),
			imageID:       params.ImageID,
		},
	}, nil
}

func (v BootcVMLinux) Run() (err error) {
	fmt.Printf("Creating VM %s\n", v.name)
	conn, err := libvirt.NewConnect("qemu:///session")
	if err != nil {
		return
	}
	defer conn.Close()

	domainXML, err := v.parseDomainTemplate()
	if err != nil {
		return fmt.Errorf("unable to parse domain template: %w", err)
	}

	v.domain, err = conn.DomainDefineXMLFlags(domainXML, libvirt.DOMAIN_DEFINE_VALIDATE)
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

func (v BootcVMLinux) parseDomainTemplate() (domainXML string, err error) {
	tmpl, err := template.New("domain-template").Parse(domainTemplate)
	if err != nil {
		return "", fmt.Errorf("unable to parse domain template: %w", err)
	}

	var domainXMLBuf bytes.Buffer

	type TemplateParams struct {
		DiskImagePath string
		Port          string
		PIDFile       string
		SMBios        string
		Name					string
	}

	templateParams := TemplateParams{
		DiskImagePath: v.diskImagePath,
		Port:          strconv.Itoa(v.sshPort),
		PIDFile:       v.pidFile,
		Name:          vmName(v.imageID),
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

	err = tmpl.Execute(&domainXMLBuf, templateParams)
	if err != nil {
		return "", fmt.Errorf("unable to execute domain template: %w", err)
	}

	return domainXMLBuf.String(), nil
}

func (v BootcVMLinux) waitForVMToBeRunning() error {
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

// Kill stops and removes the VM
func (v BootcVMLinux) Kill() error {
	if v.domain == nil {
		return fmt.Errorf("no domain to kill")
	}

	// err := v.domain.Destroy()
	// if err != nil {
	// 	return fmt.Errorf("unable to destroy VM: %w", err)
	// }
	//
	err := v.domain.UndefineFlags(libvirt.DOMAIN_UNDEFINE_NVRAM)
	if err != nil {
		return fmt.Errorf("unable to undefine VM: %w", err)
	}

	err = v.domain.Undefine()
	if err != nil {
		return fmt.Errorf("unable to undefine VM: %w", err)
	}

	return nil
}
