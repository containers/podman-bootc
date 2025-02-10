% podman-bootc 1

## NAME
podman-bootc - Run bootable containers as a virtual machine

## SYNOPSIS
**podman-bootc** [*options*] *command*

## DESCRIPTION
**podman-bootc** is a tool to streamline the local development cycle when working with bootable containers.
It makes it easy to run a local bootc image and get shell access to it without first setting up a virtual machine.

podman-bootc requires a rootful podman machine to be running before running a bootable container.
A machine can be set up using e.g. `podman machine init --rootful --now`. 
See `podman-machine(1)` for details.

**podman-bootc [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

#### **--help**, **-h**
Print usage statement

#### **--log-level**=*level*
Log messages at and above specified level: __debug__, __info__, __warn__, __error__, __fatal__ or __panic__ (default: _warn_)

## COMMANDS

| Command                                                    | Description                                                |
|------------------------------------------------------------|------------------------------------------------------------|
| [podman-bootc-completion(1)](podman-bootc-completion.1.md) | Generate the autocompletion script for the specified shell |
| [podman-bootc-images(1)](podman-bootc-images.1.md)         | List bootc images in the local containers store            |
| [podman-bootc-list(1)](podman-bootc-list.1.md)             | List installed OS Containers                               |
| [podman-bootc-rm(1)](podman-bootc-rm.1.md)                 | Remove installed bootc VMs                                 |
| [podman-bootc-run(1)](podman-bootc-run.1.md)               | Run a bootc container as a VM                              |
| [podman-bootc-ssh(1)](podman-bootc-ssh.1.md)               | SSH into an existing OS Container machine                  |
| [podman-bootc-stop(1)](podman-bootc-stop.1.md)             | Stop an existing OS Container machine                      |

## SEE ALSO
**[podman-machine(1)](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-machine.1.md)**

## HISTORY
Dec, 2024, Originally compiled by Martin Skøtt <mskoett@redhat.com>
