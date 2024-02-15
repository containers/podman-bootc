# Streamlining podman + bootc interactions

This project aims to address <https://github.com/containers/podman/issues/21243>
in alignment with the <https://github.com/containers/bootc> project.

## Goals

- Be a scriptable CLI that offers an efficient and ergonomic "edit-compile-debug" cycle for bootable containers.
- Be a backend for <https://github.com/containers/podman-desktop-extension-bootc>
- Work on both MacOS and Linux

## Setup

Requirements:

- qemu-img
- qemu-system-x86_64
- virtiofsd
- curl
- xorriso/osirrox
- golang

To compile it just run in the project directory

```shell
make
```

## Running

The core command right now is:

```shell
podmanbootc run <imagename>
```
