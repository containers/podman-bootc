#!/bin/bash -x

set -e

OSCDIR="$(dirname "$(realpath "${0}")")"

(return 0 2>/dev/null) && sourced=1 || sourced=0
if [ $sourced -eq 1 ]; then
	alias podman='${OSCDIR}/alias.sh'
	return
fi

if [ "${1}x" == "oscx" ]; then
	shift
	"${OSCDIR}"/osc "$@"
	exit 0
fi

podman "$@"
