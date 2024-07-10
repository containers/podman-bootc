package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman-bootc/pkg/utils"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/distribution/reference"
	"github.com/docker/go-units"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	imagesCmd = &cobra.Command{
		Use:   "images",
		Short: "List bootc images in the local containers store",
		Long:  "List bootc images in the local container store",
		RunE:  doImages,
	}
)

func init() {
	RootCmd.AddCommand(imagesCmd)
}

func doImages(flags *cobra.Command, args []string) error {
	machine, err := utils.GetMachineContext()
	if err != nil {
		println(utils.PodmanMachineErrorMessage)
		logrus.Errorf("failed to connect to podman machine. Is podman machine running?\n%s", err)
		return err
	}

	filters := map[string][]string{"label": []string{"containers.bootc=1"}}
	imageList, err := images.List(machine.Ctx, new(images.ListOptions).WithFilters(filters))
	if err != nil {
		return err
	}

	imageReports, err := sortImages(imageList)
	if err != nil {
		return err
	}

	return writeImagesTemplate(imageReports)
}

func writeImagesTemplate(imgs []imageReporter) error {
	hdrs := report.Headers(imageReporter{}, map[string]string{
		"ID":       "IMAGE ID",
		"ReadOnly": "R/O",
	})

	rpt := report.New(os.Stdout, "images")
	defer rpt.Flush()

	rpt, err := rpt.Parse(report.OriginPodman, lsFormatFromFlags())
	if err != nil {
		return err
	}

	if err := rpt.Execute(hdrs); err != nil {
		return err
	}

	return rpt.Execute(imgs)
}

func sortImages(imageS []*entities.ImageSummary) ([]imageReporter, error) {
	imgs := make([]imageReporter, 0, len(imageS))
	var err error
	for _, e := range imageS {
		var h imageReporter
		if len(e.RepoTags) > 0 {
			tagged := []imageReporter{}
			untagged := []imageReporter{}
			for _, tag := range e.RepoTags {
				h.ImageSummary = *e
				h.Repository, h.Tag, err = tokenRepoTag(tag)
				if err != nil {
					return nil, fmt.Errorf("parsing repository tag: %q: %w", tag, err)
				}
				if h.Tag == "<none>" {
					untagged = append(untagged, h)
				} else {
					tagged = append(tagged, h)
				}
			}
			// Note: we only want to display "<none>" if we
			// couldn't find any tagged name in RepoTags.
			if len(tagged) > 0 {
				imgs = append(imgs, tagged...)
			} else {
				imgs = append(imgs, untagged[0])
			}
		} else {
			h.ImageSummary = *e
			h.Repository = "<none>"
			h.Tag = "<none>"
			imgs = append(imgs, h)
		}
	}

	sort.Slice(imgs, sortFunc("created", imgs))
	return imgs, err
}

func tokenRepoTag(ref string) (string, string, error) {
	if ref == "<none>:<none>" {
		return "<none>", "<none>", nil
	}

	repo, err := reference.Parse(ref)
	if err != nil {
		return "<none>", "<none>", err
	}

	named, ok := repo.(reference.Named)
	if !ok {
		return ref, "<none>", nil
	}
	name := named.Name()
	if name == "" {
		name = "<none>"
	}

	tagged, ok := repo.(reference.Tagged)
	if !ok {
		return name, "<none>", nil
	}
	tag := tagged.Tag()
	if tag == "" {
		tag = "<none>"
	}

	return name, tag, nil
}

func sortFunc(key string, data []imageReporter) func(i, j int) bool {
	switch key {
	case "id":
		return func(i, j int) bool {
			return data[i].ID() < data[j].ID()
		}
	case "repository":
		return func(i, j int) bool {
			return data[i].Repository < data[j].Repository
		}
	case "size":
		return func(i, j int) bool {
			return data[i].size() < data[j].size()
		}
	case "tag":
		return func(i, j int) bool {
			return data[i].Tag < data[j].Tag
		}
	default:
		// case "created":
		return func(i, j int) bool {
			return data[i].created().After(data[j].created())
		}
	}
}

func lsFormatFromFlags() string {
	row := []string{
		"{{if .Repository}}{{.Repository}}{{else}}<none>{{end}}",
		"{{if .Tag}}{{.Tag}}{{else}}<none>{{end}}",
		"{{.ID}}", "{{.Created}}", "{{.Size}}",
	}
	return "{{range . }}" + strings.Join(row, "\t") + "\n{{end -}}"
}

type imageReporter struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	entities.ImageSummary
}

func (i imageReporter) ID() string {
	return i.ImageSummary.ID[0:12]
}

func (i imageReporter) Created() string {
	return units.HumanDuration(time.Since(i.created())) + " ago"
}

func (i imageReporter) created() time.Time {
	return time.Unix(i.ImageSummary.Created, 0).UTC()
}

func (i imageReporter) Size() string {
	s := units.HumanSizeWithPrecision(float64(i.ImageSummary.Size), 3)
	j := strings.LastIndexFunc(s, unicode.IsNumber)
	return s[:j+1] + " " + s[j+1:]
}

func (i imageReporter) History() string {
	return strings.Join(i.ImageSummary.History, ", ")
}

func (i imageReporter) CreatedAt() string {
	return i.created().String()
}

func (i imageReporter) CreatedSince() string {
	return i.Created()
}

func (i imageReporter) CreatedTime() string {
	return i.CreatedAt()
}

func (i imageReporter) size() int64 {
	return i.ImageSummary.Size
}
