#!/bin/bash

set -xe
mkdir -p /usr/lib/bootc/config
mkdir -p /usr/lib/bootc/container_storage
mkdir -p /usr/lib/bootc/output
mount -t virtiofs config /usr/lib/bootc/config
mount -t virtiofs storage /usr/lib/bootc/container_storage
mount -t virtiofs output /usr/lib/bootc/output
