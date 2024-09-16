//go:build linux

package vm_test

import (
	"context"
	"os"
	osUser "os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/podman-bootc/cmd"
	"github.com/containers/podman-bootc/pkg/bootc"
	"github.com/containers/podman-bootc/pkg/user"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman-bootc/pkg/vm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"libvirt.org/go/libvirt"
)

func TestPodmanBootc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Test Suite")
}

func projectRoot() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(ex)
	return projectRoot
}

var testUser = user.User{
	OSUser: &osUser.User{
		Uid:      "1000",
		Gid:      "1000",
		Username: "test",
		Name:     "test",
		HomeDir:  filepath.Join(projectRoot(), ".test-user-home"),
	},
}

const (
	testImageID    = "a025064b145ed339eeef86046aea3ee221a2a5a16f588aff4f43a42e5ca9f844"
	testRepoTag    = "quay.io/test/test:latest"
	testLibvirtUri = "test:///default"
)

var testUserSSHKey = filepath.Join(testUser.SSHDir(), "podman-machine-default")

var _ = BeforeSuite(func() {
	// populate the test user home directory.
	// This is most likely temporary. It enables the VM tests
	// to run, but there is propably a better solution that can be used
	// for other tests (e.g. disk image)
	err := os.MkdirAll(testUser.HomeDir(), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(testUser.SSHDir(), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(testUserSSHKey, []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(testUserSSHKey+".pub", []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(filepath.Join(testUser.HomeDir(), ".local/share/containers/podman/machine/qemu"), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(filepath.Join(testUser.HomeDir(), ".local/share/containers/podman/machine/qemu/podman.sock"), []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
})

var _ = AfterSuite(func() {
	err := os.RemoveAll(testUser.HomeDir())
	Expect(err).To(Not(HaveOccurred()))
})

func createTestVM(imageId string) (bootcVM *vm.BootcVMLinux) {
	err := os.MkdirAll(filepath.Join(testUser.CacheDir(), imageId), 0700)
	Expect(err).To(Not(HaveOccurred()))

	bootcVM, err = vm.NewVM(vm.NewVMParameters{
		ImageID:    imageId,
		User:       testUser,
		LibvirtUri: testLibvirtUri,
		Locking:    utils.Shared,
	})
	Expect(err).To(Not(HaveOccurred()))

	return
}

func runTestVM(bootcVM vm.BootcVM) {
	err := bootcVM.Run(vm.RunVMParameters{
		VMUser:        "root",
		CloudInitDir:  "",
		NoCredentials: false,
		CloudInitData: false,
		SSHPort:       22,
		Cmd:           []string{},
		RemoveVm:      false,
		Interactive:   true,
		SSHIdentity:   testUserSSHKey,
	})
	Expect(err).To(Not(HaveOccurred()))

	now := time.Now()
	now = now.Add(-time.Duration(1 * time.Minute))
	bootcDisk := bootc.BootcDisk{
		ImageNameOrId: testImageID,
		User:          testUser,
		Ctx:           context.Background(),
		ImageId:       testImageID,
		RepoTag:       testRepoTag,
		CreatedAt:     now,
		Directory:     filepath.Join(testUser.CacheDir(), testImageID),
	}

	err = os.WriteFile(filepath.Join(testUser.CacheDir(), testImageID, "disk.raw"), []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))

	err = bootcVM.WriteConfig(bootcDisk)
	Expect(err).To(Not(HaveOccurred()))
}

func deleteAllVMs() {
	conn, err := libvirt.NewConnect("test:///default")
	Expect(err).To(Not(HaveOccurred()))
	defer conn.Close()

	var flags libvirt.ConnectListAllDomainsFlags
	domains, err := conn.ListAllDomains(flags)
	Expect(err).To(Not(HaveOccurred()))
	for _, domain := range domains {
		err = domain.Destroy()
		Expect(err).To(Not(HaveOccurred()))
		err = domain.Undefine()
		Expect(err).To(Not(HaveOccurred()))
	}
}

var _ = Describe("VM", func() {
	AfterEach(func() {
		deleteAllVMs()
		err := testUser.RemoveOSCDirs()
		Expect(err).To(Not(HaveOccurred()))
	})

	BeforeEach(func() {
		err := testUser.InitOSCDirs()
		Expect(err).To(Not(HaveOccurred()))
	})

	Context("does not exist", func() {
		It("should create and start the VM after calling Run", func() {
			bootcVM := createTestVM(testImageID)
			defer func() {
				_ = bootcVM.Unlock()
			}()

			runTestVM(bootcVM)
			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeTrue())

			isRunning, err := bootcVM.IsRunning()
			Expect(err).To(Not(HaveOccurred()))
			Expect(isRunning).To(BeTrue())
		})

		It("should return false when calling Exists before Run", func() {
			bootcVM := createTestVM(testImageID)
			defer func() {
				_ = bootcVM.Unlock()
			}()

			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeFalse())
		})

		It("should return an empty list when listing", func() {
			vmList, err := cmd.CollectVmList(testUser, testLibvirtUri)
			Expect(err).To(Not(HaveOccurred()))
			Expect(vmList).To(HaveLen(0))
		})
	})

	Context("is running", func() {
		It("should remove the VM from the hypervisor after calling Delete", func() {
			//create vm and start it
			bootcVM := createTestVM(testImageID)
			defer func() {
				_ = bootcVM.Unlock()
			}()

			runTestVM(bootcVM)

			//assert that the VM exists
			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeTrue())

			//attempt to stop and delete the VM
			err = bootcVM.Delete()
			Expect(err).To(Not(HaveOccurred()))

			//assert that the VM is stopped and deleted
			exists, err = bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeFalse())
		})

		It("should list the VM", func() {
			bootcVM := createTestVM(testImageID)
			defer func() {
				_ = bootcVM.Unlock()
			}()

			runTestVM(bootcVM)
			vmList, err := cmd.CollectVmList(testUser, testLibvirtUri)
			Expect(err).To(Not(HaveOccurred()))

			Expect(vmList).To(HaveLen(1))
			Expect(vmList[0]).To(Equal(vm.BootcVMConfig{
				Id:          testImageID[:12],
				SshPort:     22,
				SshIdentity: testUserSSHKey,
				RepoTag:     testRepoTag,
				Created:     "About a minute ago",
				DiskSize:    "0B",
				Running:     true,
			}))
		})
	})

	Context("multiple running", func() {
		It("should list all VMs", func() {
			bootcVM := createTestVM(testImageID)
			defer func() {
				_ = bootcVM.Unlock()
			}()

			runTestVM(bootcVM)

			id2 := "1234564b145ed339eeef86046aea3ee221a2a5a16f588aff4f43a42e5ca9f844"
			bootcVM2 := createTestVM(id2)
			defer func() {
				_ = bootcVM2.Unlock()
			}()

			runTestVM(bootcVM2)

			id3 := "2345674b145ed339eeef86046aea3ee221a2a5a16f588aff4f43a42e5ca9f844"
			bootcVM3 := createTestVM(id3)
			defer func() {
				_ = bootcVM3.Unlock()
			}()

			runTestVM(bootcVM3)

			vmList, err := cmd.CollectVmList(testUser, testLibvirtUri)
			Expect(err).To(Not(HaveOccurred()))

			Expect(vmList).To(HaveLen(3))
			Expect(vmList).To(ContainElement(vm.BootcVMConfig{
				Id:          testImageID[:12],
				SshPort:     22,
				SshIdentity: testUserSSHKey,
				RepoTag:     testRepoTag,
				Created:     "About a minute ago",
				DiskSize:    "0B",
				Running:     true,
			}))

			Expect(vmList).To(ContainElement(vm.BootcVMConfig{
				Id:          id2[:12],
				SshPort:     22,
				SshIdentity: testUserSSHKey,
				RepoTag:     testRepoTag,
				Created:     "About a minute ago",
				DiskSize:    "0B",
				Running:     true,
			}))

			Expect(vmList).To(ContainElement(vm.BootcVMConfig{
				Id:          id3[:12],
				SshPort:     22,
				SshIdentity: testUserSSHKey,
				RepoTag:     testRepoTag,
				Created:     "About a minute ago",
				DiskSize:    "0B",
				Running:     true,
			}))
		})
	})
})
