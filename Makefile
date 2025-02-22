binary_name = podman-bootc
output_dir = bin
build_tags = exclude_graphdriver_btrfs,btrfs_noversion,exclude_graphdriver_devicemapper,containers_image_openpgp,remote

all: out_dir docs
	go build -tags $(build_tags) $(GOOPTS) -o $(output_dir)/$(binary_name)

out_dir:
	mkdir -p $(output_dir)

lint: validate_docs
	golangci-lint --build-tags $(build_tags) run

integration_tests:
	ginkgo run -tags $(build_tags) --skip-package test ./...

# !! These tests will modify your system's resources. See note in e2e_test.go. !!
e2e_test: all
	ginkgo -tags $(build_tags) ./test/...

.PHONY: docs
docs:
	make -C docs

clean:
	rm -f $(output_dir)/*
	make -C docs clean

.PHONY: validate_docs
validate_docs:
	hack/man-page-checker
	hack/xref-helpmsgs-manpages
