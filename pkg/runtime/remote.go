package runtime

import (
	"context"

	"github.com/google/go-containerregistry/pkg/crane"
)

func ListTags(ctx context.Context, src string) ([]string, error) {
	listTagsOptions := []crane.Option{
		crane.WithContext(ctx),
	}
	return crane.ListTags(src, listTagsOptions...)
}
