package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/podman-bootc/pkg/user"
)

const DefaultBaseImage = "quay.io/centos-bootc/centos-bootc-dev:stream9"
const TestImageOne = "quay.io/ckyrouac/podman-bootc-test:one"
const TestImageTwo = "quay.io/ckyrouac/podman-bootc-test:two"

var BaseImage = GetBaseImage()

func GetBaseImage() string {
	if os.Getenv("BASE_IMAGE") != "" {
		return os.Getenv("BASE_IMAGE")
	} else {
		return DefaultBaseImage
	}
}

func PodmanBootcBinary() string {
	return ProjectRoot() + "/../../bin/podman-bootc"
}

func ProjectRoot() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(ex)
	return projectRoot
}

func RunCmd(cmd string, args ...string) (stdout string, stderr string, err error) {
	execCmd := exec.Command(cmd, args...)

	var stdOut strings.Builder
	execCmd.Stdout = &stdOut

	var stdErr strings.Builder
	execCmd.Stderr = &stdErr

	err = execCmd.Run()
	if err != nil {
		println(stdOut.String())
		println(stdErr.String())
		return
	}

	return stdOut.String(), stdErr.String(), nil
}

func RunPodmanBootc(args ...string) (stdout string, stderr string, err error) {
	return RunCmd(PodmanBootcBinary(), args...)
}

func RunPodman(args ...string) (stdout string, stderr string, err error) {
	podmanArgs := append([]string{"-c", "podman-machine-default-root"}, args...)
	return RunCmd("podman", podmanArgs...)
}

func ListCacheDirs() (vmDirs []string, err error) {
	user, err := user.NewUser()
	if err != nil {
		return
	}
	cacheDirContents, err := os.ReadDir(user.CacheDir())
	if err != nil {
		return
	}

	for _, dir := range cacheDirContents {
		if dir.IsDir() {
			vmDirs = append(vmDirs, filepath.Join(user.CacheDir(), dir.Name()))
		}
	}
	return
}

func GetVMIdFromContainerImage(image string) (vmId string, err error) {
	imagesListOutput, _, err := RunPodman("images", image, "--format", "json")
	if err != nil {
		return
	}

	imagesList := []map[string]interface{}{}
	err = json.Unmarshal([]byte(imagesListOutput), &imagesList)
	if err != nil {
		return
	}

	if len(imagesList) != 1 {
		err = fmt.Errorf("Expected 1 image, got %d", len(imagesList))
		return
	}

	vmId = imagesList[0]["Id"].(string)[:12]
	return
}

func BootVM(image string) (vm *TestVM, err error) {
	runActiveCmd := exec.Command(PodmanBootcBinary(), "run", image)
	stdIn, err := runActiveCmd.StdinPipe()

	if err != nil {
		return
	}

	vm = &TestVM{
		StdIn: stdIn,
	}

	runActiveCmd.Stdout = vm
	runActiveCmd.Stderr = vm

	go func() {
		err = runActiveCmd.Run()
		if err != nil {
			return
		}
	}()

	err = vm.WaitForBoot()

	// populate the vm id after podman-bootc run
	// so we can get the id from the pulled container image
	vmId, err := GetVMIdFromContainerImage(image)
	if err != nil {
		return
	}
	vm.Id = vmId

	return
}

func Cleanup() (err error) {
	_, _, err = RunPodmanBootc("rm", "--all", "-f")
	if err != nil {
		return
	}

	_, _, err = RunPodman("rmi", BaseImage, "-f")
	if err != nil {
		return
	}

	_, _, err = RunPodman("rmi", TestImageTwo, "-f")
	if err != nil {
		return
	}

	_, _, err = RunPodman("rmi", TestImageOne, "-f")
	if err != nil {
		return
	}

	user, err := user.NewUser()
	if err != nil {
		return
	}

	err = user.RemoveOSCDirs()
	return
}

type ListEntry struct {
	Id      string
	Repo    string
	Running string
}

// ParseListOutput parses the output of the podman bootc list command for easier comparison
func ParseListOutput(stdout string) (listOutput []ListEntry) {
	listOuputLines := strings.Split(stdout, "\n")

	for i, line := range listOuputLines {
		if i == 0 {
			continue //skip the header
		}

		if len(strings.Fields(line)) == 0 {
			continue //skip the empty line
		}

		entryArray := strings.Fields(line)
		entry := ListEntry{
			Id:      string(entryArray[0]),
			Repo:    string(entryArray[1]),
			Running: string(entryArray[len(entryArray)-2]),
		}

		listOutput = append(listOutput, entry)
	}

	return
}
