package vm

import (
	"fmt"
	"os"
	"time"

	"math/rand"

	"github.com/containers/podman-bootc/pkg/vm/domain"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-tools/filepath"
	"github.com/sirupsen/logrus"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

const (
	CIDInstallVM = 3
	VSOCKPort    = 1234
)

const VMImage = "quay.io/containers/bootc-vm:latest"

const (
	mac    = "52:54:00:0b:dd:1e"
	imodel = "e1000"
)

const (
	ContainerStoragePath = "/usr/lib/bootc/container_storage"
	ConfigDir            = "/usr/lib/bootc/config"
	OutputDir            = "/usr/lib/bootc/output"
	SocketDir            = "/run/podman"
	LibvirtSocketDir     = "/run/libvirt"
	BootcDir             = "/bootc-data"
)
const (
	RootTarget            = "root"
	StorageVirtiofsTarget = "storage"
	ConfigVirtiofsTarget  = "config"
	OutputVirtiofsTarget  = "output"
)

const cmdline = "console=ttyS0 rootfstype=virtiofs root=root rw init=/sbin/init panic=1"

const VNCPort int = 5959
// The virtiofs wrapper helps to launch virtiofs with the correct flags inside a container
const VsfdWrapperPath = "/usr/local/bin/virtiofsd-wrapper"

type InstallOptions struct {
	OutputImage  string
	OutputFormat domain.DiskDriverType
	Root         bool
	Kernel       string
	Initrd       string
}

type InstallVM struct {
	libvirtURI string
	socket     string
	domain     string
	opts       InstallOptions
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOQRSTUVWXYZ0123456789"

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func NewInstallVM(path string, opts InstallOptions) *InstallVM {
	mode := "session"
	if opts.Root {
		mode = "system"
	}
	uri := fmt.Sprintf("qemu:///%s?socket=%s", mode, path)
	name := "bootc-" + RandomString(5)
	return &InstallVM{
		domain:     name,
		libvirtURI: uri,
		opts:       opts,
		socket:     path,
	}
}

func (vm *InstallVM) newDomain() *libvirtxml.Domain {
	return domain.NewDomain(
		domain.WithName(vm.domain),
		domain.WithUUID(uuid.New().String()),
		domain.WithKVM(),
		domain.WithOS(),
		domain.WithMemory(2048),
		domain.WithMemoryBackingForVirtiofs(),
		domain.WithCPUHostModel(),
		domain.WithVCPUs(2),
		domain.WithSerialConsole(),
		domain.WithVSOCK(CIDInstallVM),
		domain.WithInterface(mac, imodel),
		domain.WithDisk(filepath.Join(OutputDir, vm.opts.OutputImage), "output", "vda", vm.opts.OutputFormat, domain.DiskBusVirtio),
		domain.WithFilesystem(BootcDir, RootTarget, VsfdWrapperPath),
		domain.WithFilesystem(ContainerStoragePath, StorageVirtiofsTarget, VsfdWrapperPath),
		domain.WithFilesystem(ConfigDir, ConfigVirtiofsTarget, VsfdWrapperPath),
		domain.WithFilesystem(OutputDir, OutputVirtiofsTarget, VsfdWrapperPath),
		domain.WithDirectBoot(vm.opts.Kernel, vm.opts.Initrd, cmdline),
		domain.WithVNC(VNCPort),
		domain.WIthFeatures(),
	)
}

func waitForSocket(path string, timeout time.Duration, interval time.Duration) error {
	logrus.Debugf("Wait for socket %s", path)
	start := time.Now()

	for {
		_, err := os.Stat(path)
		if err == nil {
			return nil
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("error checking file: %w", err)
		}

		if time.Since(start) > timeout {
			break
		}
		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for file: " + path)
}

func (vm *InstallVM) Run() error {
	if err := waitForSocket(vm.socket, 2*time.Minute, 1*time.Second); err != nil {
		return err
	}
	domainXML, err := vm.newDomain().Marshal()
	if err != nil {
		return err
	}
	logrus.Debugf("XML: %s", domainXML)
	conn, err := libvirt.NewConnect(vm.libvirtURI)
	if err != nil {
		return err
	}
	_, err = conn.DomainDefineXMLFlags(domainXML, libvirt.DOMAIN_DEFINE_VALIDATE)
	if err != nil {
		return fmt.Errorf("unable to define virtual machine domain: %w", err)
	}
	dom, err := conn.LookupDomainByName(vm.domain)
	if err != nil {
		return err
	}
	defer dom.Free()
	err = dom.Create()
	if err != nil {
		return fmt.Errorf("Failed to start domain: %v", err)
	}
	logrus.Debugf("Domain %s started successfully.", vm.domain)

	return nil
}

func (vm *InstallVM) Stop() error {
	conn, err := libvirt.NewConnect(vm.libvirtURI)
	if err != nil {
		return err
	}
	dom, err := conn.LookupDomainByName(vm.domain)
	if err != nil {
		return err
	}
	defer dom.Free()
	if err := dom.Destroy(); err != nil {
		logrus.Warningf("Failed to destroy the domain %s, maybe already stopped: %v", vm.domain, err)
	}
	if err := dom.Undefine(); err != nil {
		return fmt.Errorf("Undefine failed: %v", err)
	}
	logrus.Debugf("Domain %s stopped and deleted successfully", vm.domain)

	return nil
}
