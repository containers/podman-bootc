#!/usr/bin/bash

set -xe

BOOTC_ROOT=/bootc-data

# Inject the binaries, systemd and configuration files in the bootc image
mkdir -p ${BOOTC_ROOT}/etc/sysusers.d
mkdir -p ${BOOTC_ROOT}/usr/lib/containers/storage
cp /vm_files/bootc.conf ${BOOTC_ROOT}/etc/sysusers.d/bootc.conf
cp /vm_files/podman-vsock-proxy.service ${BOOTC_ROOT}/etc/systemd/system/podman-vsock-proxy.service
cp /vm_files/mount-vfsd-targets.service ${BOOTC_ROOT}/etc/systemd/system/mount-vfsd-targets.service
cp /vm_files/mount-vfsd-targets.sh ${BOOTC_ROOT}/usr/local/bin/mount-vfsd-targets.sh
cp /vm_files/container-storage.conf ${BOOTC_ROOT}/etc/containers/storage.conf
cp /vm_files/selinux-config ${BOOTC_ROOT}/etc/selinux/config
cp /vm_files/sudoers-bootc ${BOOTC_ROOT}/etc/sudoers.d/bootc
cp /usr/local/bin/vsock-proxy ${BOOTC_ROOT}/usr/local/bin/vsock-proxy

# Enable systemd services
chroot ${BOOTC_ROOT} systemctl enable mount-vfsd-targets
chroot ${BOOTC_ROOT} systemctl enable podman.socket
chroot ${BOOTC_ROOT} systemctl enable podman-vsock-proxy.service
# Create an empty password for the bootc user
entry='bootc::20266::::::'
echo $entry >> ${BOOTC_ROOT}/etc/shadow

# Start proxy the VM port 1234 to unix socket
vsock-proxy --log-level debug -s /run/podman/podman-vm.sock -p 1234 --cid 3 \
	--listen-mode unixToVsock &> /var/log/vsock-proxy.log &

# Finally, start libvirt
/usr/sbin/virtlogd &
/usr/bin/virtstoraged &
/usr/sbin/virtqemud -v -t 0
