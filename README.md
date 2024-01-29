# Os container experiment
## Setup
Requirements:
- qemu-img
- qemu-system-x86_64
- virtiofsd
- curl
- xorriso/osirrox
- golang

To compile it just run in the project directory
```shell
$ go build bootc
```
and call
```shell
$ ./prepare.sh check
$ ./prepare.sh setup
```
it will create a default podman machine and copy its imaga disc to `${HOME}/.cache/osc/machine`. It could fail to find 
the disk, in that case you need to do it manually. First, get the disk location, and copy it to `${HOME}/.cache/osc/machine/image.qcow2`

```shell
$ jq -r '.ImagePath' < ${HOME}/.config/containers/podman/machine/qemu/podman-machine-default.json 
```

the just run the machine
```shell
$ ./prepare.sh run
```
it will run qemu and listen for ssh connection on port `2222`.

last, let's add a podman connection to get the "full experience" :)
```shell
$ system connection add --default --identity ~/.ssh/podman-machine-default bootc-machine ssh://root@localhost:2222
```
this will cause podman commands with -r to run on this machine.

To simulate that this is part of podman:
```shell
$ alias podman='/path-to/osc/alias.sh'
```

```shell
$ podman bootc
Run bootc containers VMs

Usage:
  bootc [command]

Available Commands:
  boot        Boot OS Containers
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command

Flags:
  -h, --help     help for bootc
  -t, --toggle   Help message for toggle

Use "bootc [command] --help" for more information about a command.
```

```shell
$ podman -r bootc boot --help
Boot OS Containers

Usage:
bootc boot [flags]

Flags:
--cloudinit string      --cloudinit [[transport:]cloud-init data directory] (transport: cdrom | imds)
--gen-ssh-identity      --gen-ssh-identity (implies --inj-ssh-identity)
-h, --help                  help for boot
--inj-ssh-identity      --inj-ssh-identity
--ks string             --ks [kickstart file]
-r, --remote                --remote
--ssh-identity string   --ssh-identity <identity file> (default "~/.ssh/id_rsa")
-u, --user string           --user <user name> (default: root) (default "root")
```

## Installing & boot an os container from a local image

This step is optional because `podman bootc` will pull the image if not present.
```shell
$ podman pull quay.io/centos-bootc/fedora-bootc:eln
...
$ podman images
REPOSITORY                         TAG         IMAGE ID      CREATED     SIZE
quay.io/centos-bootc/fedora-bootc  eln         625405bb2004  5 days ago  1.17 GB
```

we can install it now usig the `IMAGE ID`, but let's do some modification first
```shell
$ podman run -it --name fbc-new 625405bb2004
bash-5.2# dnf -y install vim
...
Complete!
bash-5.2# exit
exit
```
```shell
$ podman commit fbc-new
Getting image source signatures
...
Writing manifest to image destination
Storing signatures
f8bf0386c5857ee9f60e3b9e90895b6867faf6a3c4c4b2540ef6339629f78c97
```
```shell
$ podman images
REPOSITORY                         TAG         IMAGE ID      CREATED         SIZE
<none>                             <none>      f8bf0386c585  44 seconds ago  1.28 GB # <--- our new custom image
quay.io/centos-bootc/fedora-bootc  eln         625405bb2004  5 days ago      1.17 GB
  
$ podman tag f8bf0386c585 fbc-new
$ podman images
REPOSITORY                         TAG         IMAGE ID      CREATED        SIZE
localhost/fbc-new                  latest      f8bf0386c585  4 minutes ago  1.28 GB
quay.io/centos-bootc/fedora-bootc  eln         625405bb2004  5 days ago     1.17 GB
```

let's install and boot our new image
```shell
$ podman bootc boot --gen-ssh-identity f8bf0386c585
...
Installation complete!
installImage elapsed:  41.181608696s
Connecting to vm 6c6c2fc015fe. To close connection, use `~.` or `exit`
Warning: your password will expire in 0 days.
[root@ibm-p8-kvm-03-guest-02 ~]#
```
with `--gen-ssh-identity`, `bootc` will create and inject a new ssh key. 
Now, we can check if our changes are present
```shell
[root@ibm-p8-kvm-03-guest-02 ~]# type vim
vim is /usr/bin/vim
[root@ibm-p8-kvm-03-guest-02 ~]# exit 
```

# Ideas

This is just a mockup from the user experience POV, the idea is also to support:
- premade disk images
- Be able to run the bootc container in background and support multiple ssh sessions. 
It would be something similar to how you would work with `podman container`. May be supporting `--rm` and `-i`.
- Caching, if the bootc oci image didn't change boot from the disk image with reinstalling it 
- remove installed bootc disk images