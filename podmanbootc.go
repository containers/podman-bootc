package main

import "podmanbootc/cmd"

func main() {
	cmd.Execute()
}

// TODO Commands
// inspect (if running send QMP commands)
// stop (try ssh poweroff first, if that fails send QMP)
// upgrade (bootc upgrade)
// rebase (ostree rebase)
// edit (edit the VM configuration, better with libvirt)
