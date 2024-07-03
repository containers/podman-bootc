# Streamlining podman + bootc interactions

This project aims to address <https://github.com/containers/podman/issues/21243>
in alignment with the <https://github.com/containers/bootc> project.

## Goals

- Be a scriptable CLI that offers an efficient and ergonomic "edit-compile-debug" cycle for bootable containers.
- Be a backend for <https://github.com/containers/podman-desktop-extension-bootc>
- Work on both MacOS and Linux

## Setup

Requirements:

- [bootc extension requirements](https://github.com/containers/podman-desktop-extension-bootc?tab=readme-ov-file#requirements)
  - (Even on Linux, you *must* set up `podman machine` with a rootful connection; see below)
  - `podman machine init --rootful --now`
- qemu-system-x86_64 / qemu-system-aarch64
- xorriso/osirrox
- golang
- libvirt-devel


To compile it, just run in the project directory:

```shell
make
```

### MacOS
On MacOS you can use homebrew to install podman-bootc

```
brew tap germag/podman-bootc
brew install podman-bootc
```

alternatively, you can download the latest development cutting-edge source

```
brew install --head podman-bootc
```

It will install xorriso and libvirt, but it doesn't install qemu.
You need to install qemu manually, using brew:
```
brew install qemu
```
or by other mean and make it available in the path.

### Fedora
For Fedora 40 and Rawhide we provide a COPR repository.
First, enable the COPR repository:

```
sudo dnf -y install 'dnf-command(copr)'
sudo dnf -y copr enable gmaglione/podman-bootc
```

then you can install `podman-bootc` as usual:

```
sudo dnf -y install podman-bootc
```


## Running

The core command right now is:

```shell
podman-bootc run <imagename>
```

This command creates a new virtual machine, backed by a persistent disk
image from a "self install" of the container image, and makes a SSH
connection to it.

This requires SSH to be enabled by default in your base image; by
default an automatically generated SSH key is injected via a systemd
credential attached to qemu.

Even after you close the SSH connection, the machine continues to run.

### Other commands:

- `podman-bootc list`: List running VMs
- `podman-bootc ssh`: Connect to a VM
- `podman-bootc rm`: Remove a VM

### Architecture

At the current time the `run` command uses a
[bootc install](https://containers.github.io/bootc/bootc-install.html)
flow - where the container installs itself executed in a privileged
mode inside the podman-machine VM.

The installation target is a raw disk image is created on the host, but loopback
mounted over virtiofs/9p from the podman-machine VM.

(The need for a real-root privileged container to write Linux filesystems is part of the
 rationale for requiring podman-machine even on Linux is that
 it keeps the architecture aligned with MacOS (where it's always required))

In the future, support for installing via [Anaconda](https://github.com/rhinstaller/anaconda/)
and [bootc-image-builder](https://github.com/osbuild/bootc-image-builder)
will be added.
