package vm_test

import (
	"os"
	osUser "os/user"
	"path/filepath"
	"podman-bootc/pkg/user"
	"testing"

	"podman-bootc/pkg/vm"

	"libvirt.org/go/libvirt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

const testImageID = "a025064b145ed339eeef86046aea3ee221a2a5a16f588aff4f43a42e5ca9f844"

var _ = BeforeSuite(func() {
	// populate the test user home directory.
	// This is most likely temporary. It enables the VM tests
	// to run, but there is propably a better solution that can be used
	// for other tests (e.g. disk image)
	err := os.MkdirAll(testUser.HomeDir(), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(testUser.SSHDir(), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(testUser.MachineSshKeyPriv(), []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(testUser.MachineSshKeyPub(), []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(filepath.Join(testUser.HomeDir(), ".local/share/containers/podman/machine/qemu"), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.WriteFile(testUser.MachineSocket(), []byte(""), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(testUser.ConfigDir(), 0700)
	Expect(err).To(Not(HaveOccurred()))
	err = os.MkdirAll(filepath.Join(testUser.CacheDir(), testImageID), 0700)
	Expect(err).To(Not(HaveOccurred()))
})

var _ = AfterSuite(func() {
	err := os.RemoveAll(testUser.HomeDir())
	Expect(err).To(Not(HaveOccurred()))
})

func createTestVM() (bootcVM *vm.BootcVMLinux) {
	bootcVM, err := vm.NewVM(vm.NewVMParameters{
		ImageID:   testImageID,
		User:       testUser,
		LibvirtUri:  "test:///default",
	})
	Expect(err).To(Not(HaveOccurred()))
	return
}

func deleteAllVMs() {
	conn, err := libvirt.NewConnect("test:///default")
	Expect(err).To(Not(HaveOccurred()))
	defer conn.Close()

	var flags libvirt.ConnectListAllDomainsFlags
	domains, err := conn.ListAllDomains(flags)
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
	})

	Context("does not exist", func() {
		It("should create and start the VM after calling Run", func() {
			bootcVM := createTestVM()
			err := bootcVM.Run(vm.RunVMParameters{
				VMUser:        "root",
				CloudInitDir:  "",
				NoCredentials: false,
				CloudInitData: false,
				SSHPort:       22,
				Cmd:           []string{},
				RemoveVm:      false,
				Background:    false,
			})
			Expect(err).To(Not(HaveOccurred()))

			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeTrue())

			isRunning, err := bootcVM.IsRunning()
			Expect(err).To(Not(HaveOccurred()))
			Expect(isRunning).To(BeTrue())
		})

		It("should return false when calling Exists before Run", func() {
			bootcVM := createTestVM()
			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeFalse())
		})
	})

	Context("is running", func() {
		It("should remove the VM from the hypervisor after calling ForceDelete", func() {
			//create vm and start it
			bootcVM := createTestVM()
			err := bootcVM.Run(vm.RunVMParameters{
				VMUser:        "root",
				CloudInitDir:  "",
				NoCredentials: false,
				CloudInitData: false,
				SSHPort:       22,
				Cmd:           []string{},
				RemoveVm:      false,
				Background:    false,
			})
			Expect(err).To(Not(HaveOccurred()))

			//assert that the VM is exists
			exists, err := bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeTrue())

			//attempt to stop and delete the VM
			err = bootcVM.ForceDelete()
			Expect(err).To(Not(HaveOccurred()))

			//assert that the VM is stopped and deleted
			exists, err = bootcVM.Exists()
			Expect(err).To(Not(HaveOccurred()))
			Expect(exists).To(BeFalse())
		})
	})
})
