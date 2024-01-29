package main

import "bootc/cmd"

func main() {
	if err := cmd.InitOSCDirs(); err != nil {
		panic(err)
	}

	cmd.Execute()
}

// TODO Commands
// inspect (if running send QMP commands)
// stop (try ssh poweroff first, if that fails send QMP)
// upgrade (bootc upgrade)
// rebase (ostree rebase)
// edit (edit the VM configuration, better with libvirt)
