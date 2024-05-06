#!/bin/bash

set -e

podman build --platform linux/amd64 -f Containerfile.1 -t quay.io/ckyrouac/podman-bootc-test:one-amd64 .
podman build --platform linux/arm64 -f Containerfile.1 -t quay.io/ckyrouac/podman-bootc-test:one-arm64 .
podman push quay.io/ckyrouac/podman-bootc-test:one-amd64
podman push quay.io/ckyrouac/podman-bootc-test:one-arm64
podman manifest create quay.io/ckyrouac/podman-bootc-test:one quay.io/ckyrouac/podman-bootc-test:one-arm64 quay.io/ckyrouac/podman-bootc-test:one-amd64
podman manifest push quay.io/ckyrouac/podman-bootc-test:one
podman manifest rm quay.io/ckyrouac/podman-bootc-test:one

podman build --platform linux/amd64 -f Containerfile.2 -t quay.io/ckyrouac/podman-bootc-test:two-amd64 .
podman build --platform linux/arm64 -f Containerfile.2 -t quay.io/ckyrouac/podman-bootc-test:two-arm64 .
podman push quay.io/ckyrouac/podman-bootc-test:two-amd64
podman push quay.io/ckyrouac/podman-bootc-test:two-arm64
podman manifest create quay.io/ckyrouac/podman-bootc-test:two quay.io/ckyrouac/podman-bootc-test:two-arm64 quay.io/ckyrouac/podman-bootc-test:two-amd64
podman manifest push quay.io/ckyrouac/podman-bootc-test:two
podman manifest rm quay.io/ckyrouac/podman-bootc-test:two
