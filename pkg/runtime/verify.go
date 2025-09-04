package runtime

import (
	"github.com/lingdie/image-manip-server/pkg/options"
)

func (r *Runtime) Verifybase(opt options.VerifyBaseOptions) error {
	ctx := r.runtimeCtx
	// Implementation goes here
	origImage, err := r.GetImage(ctx, opt.OriginalImage)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.OriginalImage, err)
		return err
	}
	baseImage, err := r.GetImage(ctx, opt.BaseImage)
	if err != nil {
		r.logger.Errorf("failed to get base image %q: %v", opt.BaseImage, err)
		return err
	}
	_, err = r.generateLayersToRebase(origImage, baseImage)
	if err != nil {
		r.logger.Errorf("stop verifying: %v", err)
		return err
	}
	r.logger.Infof("image %q is based on %q", opt.OriginalImage, opt.BaseImage)
	return nil
}
