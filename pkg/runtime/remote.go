package runtime

import (
	"context"

	"github.com/google/go-containerregistry/pkg/crane"
)

func ListTags(ctx context.Context, src string, insecure bool) ([]string, error) {
	listTagsOptions := []crane.Option{
		crane.WithContext(ctx),
	}
	if insecure {
		listTagsOptions = append(listTagsOptions, crane.Insecure)
	}
	return crane.ListTags(src, listTagsOptions...)
}
