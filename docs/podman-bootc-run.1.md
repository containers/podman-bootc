% podman-bootc-run 1

## NAME
podman-bootc-run - Run a bootc container as a VM

## SYNOPSIS
**podman-bootc run** [*options*] *image* | *id*

## DESCRIPTION
**podman-bootc run** creates a new virtual machine from a bootc container image or starts an existing one.
It then creates an SSH connection to the VM using injected credentials (see *--background* to run in the background).

The podman machine must be running to use this command.

## OPTIONS

#### **--background**, **-B**
Do not spawn SSH, run in background.

#### **--cloudinit**=**string**
--cloud-init <cloud-init data directory>

#### **--disk-size**=**string**
Allocate a disk image of this size in bytes; optionally accepts M, G, T suffixes

#### **--filesystem**=**string**
Override the root filesystem, e.g. xfs, btrfs, ext4.

#### **--quiet**
Suppress output from bootc disk creation and VM boot console

#### **--rm**
Remove the VM and it's disk image when the SSH connection exits. Cannot be used with *--background*

#### **--root-size-max**=**string**
Maximum size of root filesystem in bytes; optionally accepts M, G, T suffixes

#### **--user**, **-u**=**root** | *user name*
User name of injected user, default: root

## EXAMPLES
Create a virtual machine based on the latest bootable image from Fedora using XFS as the root filesystem.
```
$ podman-bootc run --filesystem=xfs quay.io/fedora/fedora-bootc:latest
```

Start a previously created VM, using *podman-bootc list* to find its ID.
```
$ podman-bootc list
ID            REPO                                       SIZE        CREATED        RUNNING     SSH PORT
d0300f628e13  quay.io/fedora/fedora-bootc:latest         10.7GB      4 minutes ago  false       34173
$ podman-bootc run d0300f628e13
```

## SEE ALSO

**[podman-bootc(1)](podman-bootc.1.md)**

## HISTORY
Dec, 2024, Originally compiled by Martin Skøtt <mskoett@redhat.com>
