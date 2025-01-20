% podman-bootc-rm 1

## NAME
podman-bootc-rm - Remove installed bootc VMs

## SYNOPSIS
**podman-bootc rm** *id* [*options*]

## DESCRIPTION
**podman-bootc rm** removes an installed bootc VM/container from the podman machine.

Use **[podman-bootc list](podman-bootc-list.1.md)** to find the IDs of installed VMs.

The podman machine must be running to use this command.

## OPTIONS

#### **--all**
Removes all non-running bootc VMs

#### **--force**, **-f**
Terminate a running VM

#### **--help**, **-h**
Help for rm

#### **--log-level**=*level*
Log messages at and above specified level: __debug__, __info__, __warn__, __error__, __fatal__ or __panic__ (default: _warn_)

## SEE ALSO

**[podman-bootc(1)](podman-bootc.1.md)**, **[podman-bootc-list(1)](podman-bootc-list.1.md)**

## HISTORY
Dec, 2024, Originally compiled by Martin Skøtt <mskoett@redhat.com>
