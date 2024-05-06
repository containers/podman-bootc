FROM quay.io/fedora/fedora:40 as build
RUN dnf install --setopt=install_weak_deps=0 -y \
      golang libvirt-devel git-core && \
    dnf clean all
COPY . /src
WORKDIR /src
RUN make

FROM quay.io/fedora/fedora:40
RUN dnf install --setopt=install_weak_deps=0 -y \
      podman gvisor-tap-vsock qemu-img qemu-kvm-core openssh-clients \
      libvirt-client libvirt-daemon-driver-qemu libvirt-daemon-driver-storage-disk && \
    dnf clean all
COPY --from=build /src/bin/podman-bootc /usr/bin/
COPY --from=build /src/entrypoint /usr/bin/
RUN useradd podman-bootc -d /cache
# nuke polkit tty agent to silence virsh warnings
RUN rm /usr/bin/pkttyagent
ENTRYPOINT ["/usr/bin/entrypoint"]
