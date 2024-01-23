#!/bin/bash
set -uo pipefail

CONFIGDIR="${HOME}/.config/osc/"
CACHEDIR="${HOME}/.cache/osc/"
MACHINEDIR=${CACHEDIR}/machine
RUNDIR="/run/user/$(id -u)/osc"

function check_deps() {
  declare -a required_cmds=('jq' 'podman' 'qemu-system-x86_64' '/usr/libexec/virtiofsd')
  for cmd in "${required_cmds[@]}"; do
    if ! command -v "${cmd}" &> /dev/null; then
      echo "'${cmd}' not found"
      exit 1
    fi
  done
}

function setup_fake_podman_machine() {
  echo "Creating default podman machine"
  podman machine init 2> /dev/null

  echo "Copying machine image to: ${MACHINEDIR}"
  mkdir -p "${MACHINEDIR}"
  podman machine start
  podman machine stop

  MACHINEDEF="${HOME}/.config/containers/podman/machine/qemu/podman-machine-default.json"
  MACHINEIMG=$(jq -r '.ImagePath' "${MACHINEDEF}")

  cp ${MACHINEIMG} "${MACHINEDIR}"/image.qcow2
}

function run_fake_podman_machine() {
  # Let's simulate podman machine with virtiofs sharing the osc cache dir
  # (I need to use this because my podman installation is too old)
  mkdir -p "${RUNDIR}"

  VFSDSOCK="${RUNDIR}/machine.sock"
  /usr/libexec/virtiofsd --socket-path="${VFSDSOCK}" --shared-dir="${CACHEDIR}" --cache=never --sandbox=none --syslog &

  sleep 2

  qemu-system-x86_64 \
    -machine memory-backend=mem,accel=kvm -cpu host -smp 2 \
    -m 2G -object memory-backend-file,id=mem,size=2G,mem-path=/dev/shm,share=on \
    -pidfile "${RUNDIR}/machine.pid" \
    -nic user,model=virtio-net-pci,hostfwd=tcp::2222-:22 \
    -drive if=virtio,file="${MACHINEDIR}"/image.qcow2 \
    -chardev socket,id=vfsdsock,path="${VFSDSOCK}" \
    -device vhost-user-fs-pci,id=vfsd_dev,queue-size=1024,chardev=vfsdsock,tag=osc-cache \
    -vnc :10 &
}

OPT="${1:-empty}"
case $OPT in
  check)
    check_deps
    ;;
  setup)
    setup_fake_podman_machine
    ;;
  run)
    run_fake_podman_machine
    ;;
  *)
    echo "Usage: ${0} check|setup|run"
    ;;
esac