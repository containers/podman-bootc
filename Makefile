binary_name = podman-bootc
output_dir = bin

all: out_dir
	go build --tags remote $(GOOPTS) -o $(output_dir)/$(binary_name)

out_dir:
	mkdir -p $(output_dir)

clean:
	rm -f $(output_dir)/*
