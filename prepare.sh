#!/bin/bash
set -euo pipefail

CONFIGDIR="${HOME}/.config/osc/"
CACHEDIR="${HOME}/.cache/osc/netinst"

declare -a required_cmds=('curl' 'osirrox' 'qemu-img' 'qemu-system-x86_64' '/usr/libexex/virtiofsd')
for cmd in "${required_cmds[@]}"; do
	if ! command -v "${cmd}" &> /dev/null; then
		echo "'${cmd}' not found"
		exit 1
	fi
done

echo "Creating config dir: ""${CONFIGDIR}"
mkdir -p "${CONFIGDIR}"

echo "This will install a copy of the fedora netinstall iso in ${CACHEDIR}"
mkdir -p "${CACHEDIR}"

echo "Downloading iso image"
ISOURL="https://download.fedoraproject.org/pub/fedora/linux/releases/39/Everything/x86_64/iso/Fedora-Everything-netinst-x86_64-39-1.5.iso"
curl -# -L "${ISOURL}" --output "${CACHEDIR}"/fedora-netinst.iso

echo "Extracting kernel & initrd"
cd "${CACHEDIR}"
osirrox -indev fedora-netinst.iso -extract /images/pxeboot .
