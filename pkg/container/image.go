package container

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings/images"
)

func NewContainerImage(imageNameOrId string, ctx context.Context) ContainerImage {
	return ContainerImage{
		ImageNameOrId: imageNameOrId,
		Ctx:           ctx,
	}
}

type ContainerImage struct {
	ImageNameOrId string
	Ctx           context.Context
	Id            string
	RepoTag       string
	Size          int64
	Pulled        bool
}

// pullImage fetches the container image if not present
func (p *ContainerImage) Pull() (err error) {
	pullPolicy := "missing"
	ids, err := images.Pull(p.Ctx, p.ImageNameOrId, &images.PullOptions{Policy: &pullPolicy})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	if len(ids) == 0 {
		return fmt.Errorf("no ids returned from image pull")
	}

	if len(ids) > 1 {
		return fmt.Errorf("multiple ids returned from image pull")
	}

	image, err := images.GetImage(p.Ctx, p.ImageNameOrId, &images.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}
	p.Size = image.Size
	p.Id = ids[0]
	p.RepoTag = image.RepoTags[0]
	p.Pulled = true

	return
}

func (p *ContainerImage) GetId() string {
	if !p.Pulled {
		panic("image not pulled")
	}

	return p.Id
}

func (p *ContainerImage) GetRepoTag() string {
	if !p.Pulled {
		panic("image not pulled")
	}

	return p.RepoTag
}

func (p *ContainerImage) GetSize() int64 {
	if !p.Pulled {
		panic("image not pulled")
	}

	return p.Size
}
