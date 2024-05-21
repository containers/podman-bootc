package e2e_test

// ****************************************************************************
// These are end-to-end tests that run the podman-bootc binary.
// A rootful podman machine is assumed to already be running.
// The tests interact directly with libvirt (on linux), qemu (on darwin),
// podman-bootc cache dirs, and podman images and containers.
//
// Running these tests will create/delete VMs, pull/remove podman images
// and containers, and remove the entire podman-bootc cache dir.
//
// These tests depend on the quay.io/ckyrouac/podman-bootc-test image
// which is built from the Containerfiles in the test/resources directory.
// ****************************************************************************

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gitlab.com/bootc-org/podman-bootc/pkg/config"
	"gitlab.com/bootc-org/podman-bootc/test/e2e"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPodmanBootcE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "End to End Test Suite")
}

var _ = BeforeSuite(func() {
	err := e2e.Cleanup()
	Expect(err).To(Not(HaveOccurred()))
})

var _ = AfterSuite(func() {
	err := e2e.Cleanup()
	Expect(err).To(Not(HaveOccurred()))
})

var _ = Describe("E2E", func() {
	Context("Run with no args from a fresh install", Ordered, func() {
		// Create the disk/VM once to avoid the overhead of creating it for each test
		var vm *e2e.TestVM

		BeforeAll(func() {
			var err error
			vm, err = e2e.BootVM(e2e.BaseImage)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should pull the container image", func() {
			imagesListOutput, _, err := e2e.RunPodman("images", e2e.BaseImage, "--format", "json")
			Expect(err).To(Not(HaveOccurred()))
			imagesList := []map[string]interface{}{}
			err = json.Unmarshal([]byte(imagesListOutput), &imagesList)
			Expect(err).To(Not(HaveOccurred()))
			Expect(imagesList).To(HaveLen(1))
		})

		It("should create a bootc disk image", func() {
			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmDirs).To(HaveLen(1))

			_, err = os.Stat(filepath.Join(vmDirs[0], config.DiskImage))
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should create a new virtual machine", func() {
			vmExists, err := e2e.VMExists(vm.Id)
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmExists).To(BeTrue())
		})

		It("should start an ssh session into the VM", func() {
			// Send a command to the VM and check the output
			err := vm.SendCommand("echo 'hello'", "hello")
			Expect(err).To(Not(HaveOccurred()))
			Expect(vm.StdOut[len(vm.StdOut)-1]).To(ContainSubstring("hello"))
		})

		It("should keep the VM running after the initial ssh session is closed", func() {
			vm.StdIn.Close() // this closes the ssh session

			vmIsRunning, err := e2e.VMIsRunning(vm.Id)
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmIsRunning).To(BeTrue())
		})

		It("should open a new ssh session into the VM via the ssh cmd", func() {
			_, _, err := e2e.RunPodmanBootc("ssh", vm.Id) //TODO: test the output, send a command
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should delete the VM and persist the disk image when calling stop", func() {
			_, _, err := e2e.RunPodmanBootc("stop", vm.Id)
			Expect(err).To(Not(HaveOccurred()))

			//qemu doesn't immediately stop the VM, so we need to wait for it to stop
			Eventually(func() bool {
				vmExists, err := e2e.VMExists(vm.Id)
				Expect(err).To(Not(HaveOccurred()))
				return vmExists
			}).Should(BeFalse())

			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))

			_, err = os.Stat(filepath.Join(vmDirs[0], config.DiskImage))
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should remove the disk image when calling rm", func() {
			_, _, err := e2e.RunPodmanBootc("rm", vm.Id)
			Expect(err).To(Not(HaveOccurred()))

			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))

			Expect(vmDirs).To(HaveLen(0))
		})

		It("Should recreate the disk and VM when calling run", func() {
			var err error
			vm, err = e2e.BootVM(e2e.BaseImage)
			Expect(err).To(Not(HaveOccurred()))

			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmDirs).To(HaveLen(1))

			vmExists, err := e2e.VMExists(vm.Id)
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmExists).To(BeTrue())
		})

		It("Should prevent removing a VM with an active SSH session", func() {
			_, _, err := e2e.RunPodmanBootc("rm", "-f", vm.Id)
			Expect(err).To(HaveOccurred())

			Eventually(func() int {
				vmDirs, err := e2e.ListCacheDirs()
				Expect(err).To(Not(HaveOccurred()))
				return len(vmDirs)
			}).Should(Equal(1))
		})

		It("Should remove the cache directory when calling rm -f while VM is running", func() {
			// the SSH connection needs to be closed before attempting rm -f
			err := vm.StdIn.Close()
			Expect(err).To(Not(HaveOccurred()))

			_, _, err = e2e.RunPodmanBootc("rm", "-f", vm.Id)
			Expect(err).To(Not(HaveOccurred()))

			Eventually(func() int {
				vmDirs, err := e2e.ListCacheDirs()
				Expect(err).To(Not(HaveOccurred()))
				return len(vmDirs)
			}).Should(Equal(0))
		})

		AfterAll(func() {
			vm.StdIn.Close()
			err := e2e.Cleanup()
			if err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Multiple VMs exist", Ordered, func() {
		var activeVM *e2e.TestVM
		var inactiveVM *e2e.TestVM
		var stoppedVM *e2e.TestVM

		BeforeAll(func() {
			var err error
			errors := make(chan error)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				// create an "active" VM
				// running with an active SSH session
				println("**** STARTING ACTIVE VM")
				activeVM, err = e2e.BootVM(e2e.TestImageTwo)
				if err != nil {
					errors <- err
				}
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				// create an "inactive" VM
				// running with no active SSH session
				inactiveVM, err = e2e.BootVM(e2e.TestImageOne)
				if err != nil {
					errors <- err
				}
				err = inactiveVM.StdIn.Close()
				if err != nil {
					errors <- err
				}
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				// create a "stopped" VM
				// VM does not exist but the VM directory containing the cached disk image does
				stoppedVM, err = e2e.BootVM(e2e.BaseImage)
				if err != nil {
					errors <- err
				}
				err = stoppedVM.StdIn.Close() //ssh needs to be closed before stopping the VM
				if err != nil {
					errors <- err
				}
				_, _, err = e2e.RunPodmanBootc("stop", stoppedVM.Id)
				if err != nil {
					errors <- err
				}
				wg.Done()
			}()

			wg.Wait()
			close(errors)

			if err := <-errors; err != nil {
				Fail(err.Error())
			}

			// validate there are 3 vm directories
			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmDirs).To(HaveLen(3))
		})

		It("Should list multiple VMs", func() {
			stdout, _, err := e2e.RunPodmanBootc("list")
			Expect(err).To(Not(HaveOccurred()))

			listOutput := e2e.ParseListOutput(stdout)
			Expect(listOutput).To(HaveLen(3))
			Expect(listOutput).To(ContainElement(e2e.ListEntry{
				Id:      activeVM.Id,
				Repo:    e2e.TestImageTwo,
				Running: "true",
			}))

			Expect(listOutput).To(ContainElement(e2e.ListEntry{
				Id:      inactiveVM.Id,
				Repo:    e2e.TestImageOne,
				Running: "true",
			}))

			Expect(listOutput).To(ContainElement(e2e.ListEntry{
				Id:      stoppedVM.Id,
				Repo:    e2e.BaseImage,
				Running: "false",
			}))
		})

		It("Should remove all inactive VMs and caches when calling rm -f --all", func() {
			_, _, err := e2e.RunPodmanBootc("rm", "-f", "--all")
			Expect(err).To(Not(HaveOccurred()))

			stdout, _, err := e2e.RunPodmanBootc("list")
			Expect(err).To(Not(HaveOccurred()))

			// should keep the active VM that has an ssh session open
			Expect(stdout).To(ContainSubstring(activeVM.Id))

			// should remove the other VMs
			Expect(stdout).To(Not(ContainSubstring(stoppedVM.Id)))
			Expect(stdout).To(Not(ContainSubstring(inactiveVM.Id)))

			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmDirs).To(HaveLen(1))
			Expect(vmDirs[0]).To(ContainSubstring(activeVM.Id))
		})

		It("Should no-op and return successfully when rm -f --all with no VMs", func() {
			//cleanup the remaining active VM first
			err := activeVM.StdIn.Close()
			Expect(err).To(Not(HaveOccurred()))
			_, _, err = e2e.RunPodmanBootc("rm", "-f", activeVM.Id)
			Expect(err).To(Not(HaveOccurred()))

			// verify there are no VMs
			vmDirs, err := e2e.ListCacheDirs()
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmDirs).To(HaveLen(0))

			// attempt rm -f --all
			_, _, err = e2e.RunPodmanBootc("rm", "-f", "--all")
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterAll(func() {
			activeVM.StdIn.Close()
			inactiveVM.StdIn.Close()
			stoppedVM.StdIn.Close()
			err := e2e.Cleanup()
			if err != nil {
				Fail(err.Error())
			}
		})
	})
})
