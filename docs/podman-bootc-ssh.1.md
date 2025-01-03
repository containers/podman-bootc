% podman-bootc-ssh 1

## NAME
podman-bootc-ssh - SSH into an existing OS Container machine

## SYNOPSIS
**podman-bootc ssh** *id* [*options*]

## DESCRIPTION
**podman-bootc ssh** opens an SSH connection to a running OS container machine.

Use **[podman-bootc list](podman-bootc-list.1.md)** to find the IDs of installed VMs.

## OPTIONS

#### **--user**, **-u**=**root** | *user name*
User name to use for connection, default: root

## SEE ALSO

**[podman-bootc(1)](podman-bootc.1.md)**, **[podman-bootc-list(1)](podman-bootc-list.1.md)**

## HISTORY
Dec, 2024, Originally compiled by Martin Skøtt <mskoett@redhat.com>
