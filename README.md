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
  - `podman machine init --rootful && podman machine start`
- qemu-system-x86_64/qemu-system-aarch64
- xorriso/osirrox
- golang
- libvirt-devel


To compile it just run in the project directory

```shell
make
```

On MacOS you can use homebrew to install podman-bootc

```
brew tap germag/podman-bootc
brew install podman-bootc
```

It will install xorriso and libvirt, but it doesn't install qemu.
You need to install qemu manually, using brew:
```
brew install qemu
```
or by other mean and make it available in the path.


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

## Running as a container (Linux)

There is also a container image available at `quay.io/podman/podman-bootc` that
makes it more convenient to run podman-bootc on Linux. It can run under rootless
podman. The only requirement is that `/dev/kvm` must be passed through.

In this model, the underlying `podman machine` infrastructure is automatically
brought up on startup and brought down on exit. This allows booting a bootc
container in a single command:

```shell
podman run --rm -ti --device /dev/kvm quay.io/podman/podman-bootc run <image>
```

You can also run the image without passing any command. In that case, you will
be given a shell in which the environment is set up and ready for `podman-bootc`
invocations.

### Caching

You likely will want to enable caching to avoid having to redownload/rederive
artifacts every time. To do this, mount a volume at `/cache`. For example:

```shell
# from a volume
podman volume create podman-bootc
podman run --rm -v podman-bootc:/cache --device /dev/kvm quay.io/podman/podman-bootc run <image>

# from a mountpoint
podman run --rm -v ~/.cache/podman-bootc:/cache:z  --device /dev/kvm quay.io/podman/podman-bootc
```

## Architecture

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
