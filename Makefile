binary_name = podman-bootc
output_dir = bin
build_tags = exclude_graphdriver_btrfs,btrfs_noversion,exclude_graphdriver_devicemapper,containers_image_openpgp,remote

all: out_dir
	go build -tags $(build_tags) $(GOOPTS) -o $(output_dir)/$(binary_name)

out_dir:
	mkdir -p $(output_dir)

test:
	ginkgo -tags $(build_tags) ./...

clean:
	rm -f $(output_dir)/*
