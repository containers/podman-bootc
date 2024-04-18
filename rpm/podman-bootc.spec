%bcond_without check

# https://gitlab.com/bootc-org/podman-bootc-cli
%global goipath         gitlab.com/bootc-org/podman-bootc
%global forgeurl        https://gitlab.com/bootc-org/podman-bootc-cli
Version:                0.1.1

%gometa -L -f

%global golicenses      LICENSE
%global godocs          README.md

Name:           podman-bootc
Release:        1%{?dist}
Summary:        Streamlining podman + bootc interactions

License:        Apache-2.0
URL:            %{forgeurl}
Source:         podman-bootc-0.1.1.tar.gz

%description
%{summary}

BuildRequires: gcc
BuildRequires: golang
BuildRequires: make
BuildRequires: libvirt-devel

Requires: xorriso
Requires: podman
Requires: qemu
Requires: libvirt

%gopkg

%prep
%goprep -A
%autopatch -p1

%generate_buildrequires
%go_generate_buildrequires

%build
make

%install
%gopkginstall
install -m 0755 -vd                     %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/* %{buildroot}%{_bindir}/

%if %{with check}
%check
%gocheck
%endif

%files
%license LICENSE
%doc README.md
%{_bindir}/*

%gopkgfiles

%changelog
%autochangelog
