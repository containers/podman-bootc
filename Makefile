binary_name = podman-bootc
output_dir = bin
build_tags = exclude_graphdriver_btrfs,btrfs_noversion,exclude_graphdriver_devicemapper,containers_image_openpgp,remote

all: out_dir
	go build -mod=vendor -tags $(build_tags) $(GOOPTS) -o $(output_dir)/$(binary_name)

out_dir:
	mkdir -p $(output_dir)

lint:
	golangci-lint --build-tags $(build_tags) run

integration_tests:
	ginkgo run -tags $(build_tags) --mod=vendor --skip-package test ./...

# !! These tests will modify your system's resources. See note in e2e_test.go. !!
e2e_test: all
	ginkgo -tags $(build_tags) ./test/...

clean:
	rm -f $(output_dir)/*
