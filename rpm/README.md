# Building the rpm locally

The revised spec file uses [`go-vendor-tools`]
(https://fedora.gitlab.io/sigs/go/go-vendor-tools/scenarios/#generate-specfile-with-go2rpm)
which enable vendoring for Go
language packages in Fedora.

Follow these steps to build an rpm locally using Fedora packager tools.
These steps assume a Fedora machine.

1. Install packager tools. See [Installing Packager Tools](https://docs.fedoraproject.org/en-US/package-maintainers/Installing_Packager_Tools/).

1. Install go-vendor-tools:

   ``` bash
   sudo dnf install go-vendor-tools*
   ```

1. Copy `podman-bootc.spec` and `go-vendor-tools.toml` into a directory.

1. Change into the directory.

1. Change version in spec file if needed.

1. Download tar file:

   ``` bash
   spectool -g -s 0 podman-bootc.spec
   ```

1. Generate archive:

   ```bash
   go_vendor_archive create --config go-vendor-tools.toml podman-bootc.spec
   ```

1. Build rpm locally:

   ```bash
   fedpkg --release rawhide mockbuild --srpm-mock
   ```

1. Check output in the `results_podman-bootc` subdirectory
