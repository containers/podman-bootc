#!/usr/bin/env bash

set -eox pipefail

PACKAGE=podman-bootc

# Set path to rpm spec file
SPEC_FILE=rpm/$PACKAGE.spec

# Get full version from HEAD
VERSION=$(git describe --always --long --dirty)

# RPM Version can't take "-"
RPM_VERSION="${VERSION//-/\~}"

# Generate source tarball from HEAD
RPM_SOURCE_FILE=$PACKAGE-$VERSION.tar.gz
git-archive-all -C "$(git rev-parse --show-toplevel)" --prefix="$PACKAGE-$RPM_VERSION/" "rpm/$RPM_SOURCE_FILE"

# Generate vendor dir
RPM_VENDOR_FILE=$PACKAGE-$VERSION-vendor.tar.gz
#go mod vendor
tar -czf "rpm/$RPM_VENDOR_FILE" vendor/

# RPM Spec modifications
# Use the Version from HEAD in rpm spec
sed -i "s/^Version:.*/Version: $RPM_VERSION/" $SPEC_FILE

# Use Packit's supplied variable in the Release field in rpm spec.
sed -i "s/^Release:.*/Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/" $SPEC_FILE

# Ensure last part of the release string is the git shortcommit without a prepended "g"
sed -i "/^Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/ s/\(.*\)g/\1/" $SPEC_FILE

# Use above generated tarballs as Sources in rpm spec
sed -i "s/^Source0:.*/Source0: $RPM_SOURCE_FILE/" $SPEC_FILE
sed -i "s/^Source1:.*/Source1: $RPM_VENDOR_FILE/" $SPEC_FILE
