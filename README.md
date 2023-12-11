# Os container experiment

To simulate that this is part of podman:
```shell
alias podman='/path-to/osc/alias.sh'
```

```shell
$ podman osc
Manage OS containers VMs: install, remove, update, etc. 
For example:

osc install --name fedora-base quay.io/centos-bootc/fedora-bootc:eln

Usage:
  osc [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  install     install an OS container
  list        List installed OS Containers
  rm          Remove installed OS Containers
  ssh         SSH into an existing OS Container machine
  start       Start an existing OS Container machine
  stop        Stop an existing OS Container machine

Flags:
  -h, --help     help for osc
  -t, --toggle   Help message for toggle

Use "osc [command] --help" for more information about a command.
```

## Installing an os container from a registry

List installed VMs
```shell
$ podman osc list
NAME                           		 VCPUs 		       MEMORY 		       DISK SIZE
```

let's install a new container from a registry
```shell
$ podman osc install --name fedbase quay.io/centos-bootc/fedora-bootc:eln
...
(qemu + anaconda output)
...
Installed
```
this will install `fedora-bootc:eln` in a new VM called `fedbase`

```shell
$ podman osc list
NAME                           		 VCPUs 		       MEMORY 		       DISK SIZE
fedbase                        		    2 		     2048 MiB 		          10 GiB 	    Stopped
```

now we can start the VM and enter using `ssh`

```shell
$ podman osc start fedbase
$ podman osc ssh fedbase
Connecting to vm fedbase. To close connection, use `~.` or `exit`
root@localhost#  
```

we can also send commands by `ssh` 
```shell
$ podman osc ssh fedbase -- uname -a
Linux localhost.localdomain 6.7.0-0.rc4.35.eln132.x86_64 #1 SMP PREEMPT_DYNAMIC Mon Dec  4 15:54:35 UTC 2023 x86_64 GNU/Linux
```
to stop the VM (currently it sends a `poweroff` command via `ssh`)
```shell
$ podman osc stop fedbase
```

## Installing an os container from a local image
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

let's install our new image
```shell
$ podman osc install --name fbc-custom f8bf0386c585
...
(qemu + anaconda output)
...
Installed
```

we can now check if our changes are present
```shell
$ podman osc start fbc-custom
$ podman osc ssh fbc-custom
Connecting to vm fbc-custom. To close connection, use `~.` or `exit`
root@localhost# type vim
vim is /usr/bin/vim
root@localhost# poweroff 
```
## Removing VMs

Let's remove the installed VMs
```shell
$ podman osc list
NAME                           		 VCPUs 		       MEMORY 		       DISK SIZE
fedbase                        		    2 		     2048 MiB 		          10 GiB 	    Stopped
fbc-custom                     		    2 		     2048 MiB 		          10 GiB 	    Stopped
$ podman osc rm fedbase
$ podman osc rm fbc-custom
```

