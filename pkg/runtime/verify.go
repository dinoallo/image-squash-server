package runtime

import (
	"context"
	"fmt"

	"github.com/lingdie/image-manip-server/pkg/options"
)

func (r *Runtime) Verifybase(ctx context.Context, opt options.VerifyBaseOptions) error {
	origImage, err := r.GetImage(ctx, opt.OriginalImage)
	if err != nil {
		r.Errorf("failed to get original image %q: %v", opt.OriginalImage, err)
		return err
	}
	baseImage, err := r.GetImage(ctx, opt.BaseImage)
	if err != nil {
		r.Errorf("failed to get base image %q: %v", opt.BaseImage, err)
		return err
	}
	if len(origImage.Manifest.Layers) < len(baseImage.Manifest.Layers) {
		err := fmt.Errorf("original image %q has fewer layers (%d) than base image %q (%d)", opt.OriginalImage, len(origImage.Manifest.Layers), opt.BaseImage, len(baseImage.Manifest.Layers))
		r.Error(err)
		return err
	}
	for i, baseLayer := range baseImage.Manifest.Layers {
		origLayer := origImage.Manifest.Layers[i]
		if baseLayer.Digest != origLayer.Digest {
			err := fmt.Errorf("layer %d digest mismatch: original image %q has layer digest %q, base image %q has layer digest %q", i, opt.OriginalImage, origLayer.Digest.String(), opt.BaseImage, baseLayer.Digest.String())
			r.Error(err)
			return err
		}
	}
	r.Infof("image %q is based on %q", opt.OriginalImage, opt.BaseImage)
	return nil
}
