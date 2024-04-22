package e2e

import (
	"gitlab.com/bootc-org/podman-bootc/pkg/config"

	"libvirt.org/go/libvirt"
)

func VMExists(id string) (exits bool, err error) {
	vmName := "podman-bootc-" + id

	libvirtConnection, err := libvirt.NewConnect(config.LibvirtUri)
	if err != nil {
		return false, err
	}

	defer libvirtConnection.Close()

	domains, err := libvirtConnection.ListAllDomains(libvirt.ConnectListAllDomainsFlags(0))
	if err != nil {
		return false, err
	}
	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			return false, err
		}

		if name == vmName {
			return true, nil
		}
	}

	return false, nil
}

func VMIsRunning(id string) (exits bool, err error) {
	vmName := "podman-bootc-" + id

	libvirtConnection, err := libvirt.NewConnect(config.LibvirtUri)
	if err != nil {
		return false, err
	}
	defer libvirtConnection.Close()

	domain, err := libvirtConnection.LookupDomainByName(vmName)
	if err != nil {
		return false, err
	}

	if domain == nil {
		return false, nil
	}

	state, _, err := domain.GetState()
	if err != nil {
		return false, err
	}

	if state == libvirt.DOMAIN_RUNNING {
		return true, nil
	} else {
		return false, nil
	}
}
