package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/podman-bootc/pkg/config"
	"github.com/containers/podman-bootc/pkg/define"
	"github.com/containers/podman-bootc/pkg/storage"

	"github.com/adrg/xdg"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCacheBucket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cache Bucket Suite")
}

const FakeFullId = define.FullImageId("0000000000000000000000000000000000000000000000000000000000000000") // Just 64 0's
const projectTest = config.ProjectName + "-test"

var baseDir = filepath.Join(xdg.RuntimeDir, projectTest)
var cacheDir = filepath.Join(baseDir, "cache")
var runDir = filepath.Join(baseDir, "run")
var scratchDir = filepath.Join(baseDir, "scratch")

var _ = BeforeEach(func() {
	err := os.MkdirAll(cacheDir, 0700)
	Expect(err).To(Not(HaveOccurred()))

	err = os.MkdirAll(runDir, 0700)
	Expect(err).To(Not(HaveOccurred()))

	err = os.MkdirAll(scratchDir, 0700)
	Expect(err).To(Not(HaveOccurred()))
})

var _ = AfterEach(func() {
	err := os.RemoveAll(baseDir)
	Expect(err).To(Not(HaveOccurred()))
})

var _ = Describe("Bucket", func() {
	bucket := storage.NewBucket(cacheDir, runDir)

	Context("does not exist", func() {
		It("should return an empty slice", func() {
			list, err := bucket.List()
			Expect(err).To(Not(HaveOccurred()))
			Expect(list).Should(BeEmpty())
		})

		It("should return a nil ptr", func() {
			id, err := bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(BeNil())

			// Asking for shared access
			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(roguard).Should(BeNil())
			Expect(rounlock).Should(BeNil())

			// Asking for exclusive access
			wguard, wunlock, err := bucket.GetExclusive(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(wguard).Should(BeNil())
			Expect(wunlock).Should(BeNil())
		})

		It("should create a new entry", func() {
			wguard, wunlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(wguard).Should(Not(BeNil()))
			Expect(wunlock).Should(Not(BeNil()))

			err = wunlock()
			Expect(err).To(Not(HaveOccurred()))

			list, err := bucket.List()
			Expect(err).To(Not(HaveOccurred()))
			Expect(list).Should(Not(BeEmpty()))

			// Let's search for it
			id, err := bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(Not(BeNil()))
			Expect(*id).Should(Equal(FakeFullId))

			// Asking for shared access
			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(roguard).Should(Not(BeNil()))
			Expect(rounlock).Should(Not(BeNil()))

			err = rounlock()
			Expect(err).To(Not(HaveOccurred()))

			// Asking for exclusive access
			wguard, wunlock, err = bucket.GetExclusive(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(wguard).Should(Not(BeNil()))
			Expect(wunlock).Should(Not(BeNil()))

			err = wunlock()
			Expect(err).To(Not(HaveOccurred()))
		})
	})

	Context("does exist with an exclusive access hold", func() {
		var guard *storage.WriteGuard
		var unlock storage.UnlockFunc
		var err error
		BeforeEach(func() {
			guard, unlock, err = bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))
		})

		AfterEach(func() {
			err := unlock()
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should return an non-empty slice", func() {
			list, err := bucket.List()
			Expect(err).To(Not(HaveOccurred()))
			Expect(list).Should(Not(BeEmpty()))
		})

		It("should return a non-nil ptr", func() {
			id, err := bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(Not(BeNil()))
			Expect(*id).Should(Equal(FakeFullId))
		})

		It("should return an error", func() {
			wguard, wunlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(HaveOccurred())
			Expect(wguard).Should(BeNil())
			Expect(wunlock).Should(BeNil())

			// Asking for shared access
			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(HaveOccurred())
			Expect(roguard).Should(BeNil())
			Expect(rounlock).Should(BeNil())

			// Asking for exclusive access
			wguard, wunlock, err = bucket.GetExclusive(FakeFullId)
			Expect(err).To(HaveOccurred())
			Expect(wguard).Should(BeNil())
			Expect(wunlock).Should(BeNil())
		})
	})

	Context("does exist with a shared access hold", func() {
		var guard *storage.ReadOnlyGuard
		var unlock storage.UnlockFunc

		BeforeEach(func() {
			wguard, wunlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(wguard).Should(Not(BeNil()))
			Expect(wunlock).Should(Not(BeNil()))
			_ = wunlock()

			guard, unlock, err = bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))
		})

		AfterEach(func() {
			err := unlock()
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should return an non-empty slice", func() {
			list, err := bucket.List()
			Expect(err).To(Not(HaveOccurred()))
			Expect(list).Should(Not(BeEmpty()))
		})

		It("should return a non-nil ptr", func() {
			id, err := bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(Not(BeNil()))
			Expect(*id).Should(Equal(FakeFullId))
		})

		It("should return an error", func() {
			wguard, wunlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(HaveOccurred())
			Expect(wguard).Should(BeNil())
			Expect(wunlock).Should(BeNil())

			wguard, wunlock, err = bucket.GetExclusive(FakeFullId)
			Expect(err).To(HaveOccurred())
			Expect(wguard).Should(BeNil())
			Expect(wunlock).Should(BeNil())
		})

		It("should succeed", func() {
			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(roguard).Should(Not(BeNil()))
			Expect(rounlock).Should(Not(BeNil()))

			err = rounlock()
			Expect(err).To(Not(HaveOccurred()))
		})
	})

	Context("an exclusive access hold", func() {
		It("should create a new entry and remove it", func() {
			// it should not be present
			id, err := bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(BeNil())

			// create a new entry
			guard, unlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))

			// Let's search for the new entry
			id, err = bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(Not(BeNil()))
			Expect(*id).Should(Equal(FakeFullId))

			// remove the entry
			err = guard.Remove()
			Expect(err).To(Not(HaveOccurred()))

			// it should not be present
			id, err = bucket.SearchByPrefix("0")
			Expect(err).To(Not(HaveOccurred()))
			Expect(id).Should(BeNil())

			err = unlock()
			Expect(err).To(Not(HaveOccurred()))
		})
	})

	Context("storing and extracting data", func() {
		const testFile = "config.dat"
		const tmpFile = "import.dat"

		var testData = []byte("Hello, Test")
		It("should store a file and recover its content", func() {
			guard, unlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))

			err = guard.Store(testFile, testData)
			Expect(err).To(Not(HaveOccurred()))

			err = unlock()
			Expect(err).To(Not(HaveOccurred()))

			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(roguard).Should(Not(BeNil()))
			Expect(rounlock).Should(Not(BeNil()))

			data, err := roguard.Load(testFile)
			Expect(err).To(Not(HaveOccurred()))
			Expect(data).To(Equal(testData))

			err = rounlock()
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should import a file", func() {
			// let's create a simple file
			f := filepath.Join(scratchDir, tmpFile)
			err := os.WriteFile(f, []byte("xxx"), 0660)
			Expect(err).To(Not(HaveOccurred()))

			// Import
			guard, unlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))

			err = guard.MoveIntoRename(f, tmpFile)
			Expect(err).To(Not(HaveOccurred()))

			// the file should be moved into the storage
			_, err = os.Stat(f)
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(filepath.Join(cacheDir, string(FakeFullId), tmpFile))
			Expect(err).To(Not(HaveOccurred()))

			err = unlock()
			Expect(err).To(Not(HaveOccurred()))
		})
	})

	Context("invalid guard", func() {
		It("should panic", func() {
			// Exclusive guard
			guard, unlock, err := bucket.GetExclusiveOrAdd(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(guard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))

			err = unlock()
			Expect(err).To(Not(HaveOccurred()))

			Expect(func() { _, _ = guard.FilePath("xxx") }).To(Panic())
			Expect(func() { _ = guard.Store("yyy", []byte("xxx")) }).To(Panic())
			Expect(func() { _ = guard.MoveIntoRename("src", "dst") }).To(Panic())
			Expect(func() { _ = guard.Remove() }).To(Panic())

			// Shared guard
			roguard, rounlock, err := bucket.Get(FakeFullId)
			Expect(err).To(Not(HaveOccurred()))
			Expect(roguard).Should(Not(BeNil()))
			Expect(unlock).Should(Not(BeNil()))

			err = rounlock()
			Expect(err).To(Not(HaveOccurred()))
			Expect(func() { _, _ = roguard.Load("yyy") }).To(Panic())
		})
	})
})
