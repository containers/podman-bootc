#!/bin/bash

set -e

BOOTCDIR="$(dirname "$(realpath "${0}")")"

(return 0 2>/dev/null) && sourced=1 || sourced=0
if [ $sourced -eq 1 ]; then
	alias podman='${BOOTCDIR}/alias.sh'
	return
fi

if [ "${1}x" == "bootcx" ]; then
	shift
	"${BOOTCDIR}"/bootc "$@"
	exit 0
fi

if [ "${1}x" == "-rx" ] || [ "${1}x" == "--remotex" ]; then
  if [ "${2}x" == "bootcx" ]; then
  	shift
  	shift
  	"${BOOTCDIR}"/bootc "$@" --remote
  	exit 0
  fi
fi


podman "$@"
