package domain

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/sirupsen/logrus"
	"libvirt.org/go/libvirtxml"
)

type DomainOption func(d *libvirtxml.Domain)

const (
	MemoryMemfd            = "memfd"
	MemoryAccessModeShared = "shared"
)

type DiskDriverType string

func (d DiskDriverType) String() string {
	return string(d)
}

const (
	DiskDriverQCOW2 DiskDriverType = "qcow2"
	DiskDriverRaw   DiskDriverType = "raw"
)

type DiskBus string

func (b DiskBus) String() string {
	return string(b)
}

const (
	DiskBusSCSI   DiskBus = "scsi"
	DiskBusVirtio DiskBus = "virtio"
)

func NewDomain(opts ...DomainOption) *libvirtxml.Domain {
	domain := &libvirtxml.Domain{}
	for _, f := range opts {
		f(domain)
	}

	return domain
}

func WithName(name string) DomainOption {
	return func(d *libvirtxml.Domain) {
		d.Name = name
	}
}

func WithMemory(memory uint) DomainOption {
	return func(d *libvirtxml.Domain) {
		d.Memory = &libvirtxml.DomainMemory{
			Value: memory,
			Unit:  "MiB",
		}
	}
}

func WithMemoryBackingForVirtiofs() DomainOption {
	return func(d *libvirtxml.Domain) {
		d.MemoryBacking = &libvirtxml.DomainMemoryBacking{
			MemorySource: &libvirtxml.DomainMemorySource{Type: MemoryMemfd},
			MemoryAccess: &libvirtxml.DomainMemoryAccess{Mode: MemoryAccessModeShared},
		}
	}
}

func WithCPUHostModel() DomainOption {
	return func(d *libvirtxml.Domain) {
		d.CPU = &libvirtxml.DomainCPU{
			Mode: "host-model",
		}
	}
}

func WithVCPUs(cpus uint) DomainOption {
	return func(d *libvirtxml.Domain) {
		d.VCPU = &libvirtxml.DomainVCPU{Value: cpus}
	}
}

func allocateDevices(d *libvirtxml.Domain) {
	if d.Devices == nil {
		d.Devices = &libvirtxml.DomainDeviceList{}
	}
}

func WithFilesystem(source, target, vfsdBin string) DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.Filesystems = append(d.Devices.Filesystems, libvirtxml.DomainFilesystem{
			Driver: &libvirtxml.DomainFilesystemDriver{
				Type: "virtiofs",
			},
			Source: &libvirtxml.DomainFilesystemSource{
				Mount: &libvirtxml.DomainFilesystemSourceMount{
					Dir: source,
				},
			},
			Target: &libvirtxml.DomainFilesystemTarget{
				Dir: target,
			},
			Binary: &libvirtxml.DomainFilesystemBinary{
				Path: vfsdBin,
			},
		})
	}
}

func WithDisk(path, serial, dev string, diskType DiskDriverType, bus DiskBus) DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.Disks = append(d.Devices.Disks, libvirtxml.DomainDisk{
			Device: "disk",
			Driver: &libvirtxml.DomainDiskDriver{
				Name: "qemu",
				Type: diskType.String(),
			},
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: path,
				},
			},
			Target: &libvirtxml.DomainDiskTarget{
				Bus: bus.String(),
				Dev: dev,
			},
			Serial: serial,
		})
	}
}

func WithSerialConsole() DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.Consoles = append(d.Devices.Consoles, libvirtxml.DomainConsole{
			Source: &libvirtxml.DomainChardevSource{Pty: &libvirtxml.DomainChardevSourcePty{}},
			Target: &libvirtxml.DomainConsoleTarget{
				Type: "serial",
			},
		})

	}
}

func WithInterface(mac, model string) DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.Interfaces = append(d.Devices.Interfaces, libvirtxml.DomainInterface{
			Source: &libvirtxml.DomainInterfaceSource{
				User: &libvirtxml.DomainInterfaceSourceUser{},
			},
			MAC: &libvirtxml.DomainInterfaceMAC{
				Address: mac,
			},
			Model: &libvirtxml.DomainInterfaceModel{
				Type: model,
			},
		})
	}
}

func WithVSOCK(cid uint) DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.VSock = &libvirtxml.DomainVSock{
			Model: "virtio",
			CID: &libvirtxml.DomainVSockCID{
				Address: fmt.Sprintf("%d", cid),
			},
		}
	}
}

func WithUUID(uuid string) DomainOption {
	return func(d *libvirtxml.Domain) {
		d.UUID = uuid
	}
}

func WithKVM() DomainOption {
	return func(d *libvirtxml.Domain) {
		d.Type = "kvm"
	}
}

func WithOS() DomainOption {
	// TODO: fix this for multiarch
	return func(d *libvirtxml.Domain) {
		d.OS = &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Arch:    "x86_64",
				Machine: "q35",
				Type:    "hvm",
			},
		}
	}
}

func WithDirectBoot(kernel, initrd, cmdline string) DomainOption {
	return func(d *libvirtxml.Domain) {
		d.OS.Kernel = kernel
		d.OS.Initrd = initrd
		d.OS.Cmdline = cmdline
	}
}

func WithVNC(port int) DomainOption {
	return func(d *libvirtxml.Domain) {
		allocateDevices(d)
		d.Devices.Graphics = append(d.Devices.Graphics, libvirtxml.DomainGraphic{
			VNC: &libvirtxml.DomainGraphicVNC{
				Port:   port,
				Listen: "0.0.0.0",
			},
		})
		d.Devices.Videos = append(d.Devices.Videos, libvirtxml.DomainVideo{
			Model: libvirtxml.DomainVideoModel{
				Type: "vga",
			},
		})
	}
}

func WIthFeatures() DomainOption {
	return func(d *libvirtxml.Domain) {
		d.Features = &libvirtxml.DomainFeatureList{
			ACPI: &libvirtxml.DomainFeature{},
			APIC: &libvirtxml.DomainFeatureAPIC{},
		}
	}
}

type diskInfo struct {
	Format      string `json:"format"`
	BackingFile string `json:"backing-filename"`
	ActualSize  int64  `json:"actual-size"`
	VirtualSize int64  `json:"virtual-size"`
}

func GetDiskInfo(imagePath string) (DiskDriverType, error) {
	path, err := exec.LookPath("qemu-img")
	if err != nil {
		return "", fmt.Errorf("qemu-img not found: %v\n", err)
	}

	args := []string{"info", imagePath, "--output", "json"}
	cmd := exec.Command(path, args...)
	logrus.Debugf("Execute: %s", cmd.String())
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr for qemu-img command: %v", err)
	}
	out, err := cmd.Output()
	if err != nil {
		errout, _ := io.ReadAll(stderr)
		return "", fmt.Errorf("failed to invoke qemu-img on %s: %v: %s", imagePath, err, errout)
	}
	info := &diskInfo{}
	err = json.Unmarshal(out, info)
	if err != nil {
		return "", fmt.Errorf("failed to parse disk info: %v", err)
	}
	switch info.Format {
	case "qcow2":
		return DiskDriverQCOW2, nil
	case "raw":
		return DiskDriverRaw, nil
	default:
		return "", fmt.Errorf("Unsupported format: %s", info.Format)
	}
}
