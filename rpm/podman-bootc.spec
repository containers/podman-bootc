# https://github.com/containers/podman-bootc
%global goipath         github.com/containers/podman-bootc
Version:                0.1.1

%gometa -L -f

%global golicenses      LICENSE
%global godocs          README.md

Name:           podman-bootc
Release:        %autorelease
Summary:        Streamlining podman + bootc interactions

License:        Apache-2.0
URL:            %{gourl}
Source0:        %{gosource}
Source1:        vendor.tar.gz # Vendor file place holder

BuildRequires: gcc
BuildRequires: golang
BuildRequires: make
BuildRequires: libvirt-devel

Requires: podman-machine
Requires: xorriso
Requires: podman
Requires: qemu
Requires: libvirt

%description
%{summary}.

%gopkg

%prep
%goprep -Ak  # k: keep vendor directory
%setup -T -D -a 1
%autopatch -p1

%build
export BUILDTAGS="exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp remote"
%gobuild -o %{gobuilddir}/bin/%%{name} %{goipath}

%install
%gopkginstall
install -m 0755 -vd                     %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/* %{buildroot}%{_bindir}/

%files
%license LICENSE
%doc README.md
%{_bindir}/*

%gopkgfiles

%changelog
%autochangelog
