package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/util"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r *Runtime) Remove(ctx context.Context, opt options.RemoveOptions) error {
	// get the original image
	r.logger.Infof("start to remove file %q from image %q", opt.File, opt.ImageRef)
	defer r.Track(time.Now(), "remove")
	image, err := r.GetImage(ctx, opt.ImageRef)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.ImageRef, err)
		return err
	}
	layer, err := r.createRemovalLayer(ctx, image.Config, opt.File)
	if err != nil {
		r.logger.Errorf("failed to create removal layer for file %q: %v", opt.File, err)
		return err
	}
	newLayers := NewLayersFromLayer(layer)
	baseLayers := image.Manifest.Layers
	manifestDesc, err := r.WriteBack(ctx, image.Config, baseLayers, newLayers)
	if err != nil {
		r.logger.Errorf("failed to write back image %q: %v", opt.ImageRef, err)
		return err
	}
	var newImageName string
	// determine the new image name
	// if NewImageName is not specified, use the original image name
	if opt.NewImageName != "" {
		newImageName = opt.NewImageName
	} else {
		newImageName = image.Image.Name
	}
	img := images.Image{
		Name:      newImageName,
		Target:    manifestDesc,
		UpdatedAt: time.Now(),
	}
	img, err = r.UpdateImage(ctx, img)
	if err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", newImageName, err)
		return err
	}
	if err := r.UnpackImage(ctx, img, manifestDesc); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", newImageName, err)
		return err
	}
	r.logger.Infof("file %q removed from image %q successfully", opt.File, opt.ImageRef)
	return nil
}

func (r *Runtime) createRemovalLayer(ctx context.Context, origImage ocispec.Image, file string) (Layer, error) {
	var (
		key           = fmt.Sprintf("file-removal-%s", util.UniquePart())
		parentDiffIDs = origImage.RootFS.DiffIDs
		parent        = identity.ChainID(origImage.RootFS.DiffIDs)
		layer         = NewLayer(ocispec.Descriptor{}, digest.Digest(""))
	)
	// create mount target to mount the rootfs
	mountTarget, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "remove-file-")
	if err != nil {
		r.logger.Errorf("failed to create mount target %q: %v", mountTarget, err)
		return layer, err
	}
	defer os.RemoveAll(mountTarget)
	// prepare a temporary rootfs
	mounts, err := r.snapshotter.Prepare(ctx, key, parent.String())
	if err != nil {
		r.logger.Errorf("failed to prepare snapshot %q: %v", key, err)
		return layer, err
	}
	mounter := NewMounterImpl()
	if err := mounter.Mount(mountTarget, mounts...); err != nil {
		r.logger.Errorf("failed to mount rootfs %q: %v", mountTarget, err)
		return layer, err
	}
	defer mounter.Unmount(mountTarget)
	// remove the file
	if err := os.RemoveAll(filepath.Join(mountTarget, file)); err != nil {
		r.logger.Errorf("failed to remove file %q: %v", file, err)
		return layer, err
	}
	// create a diff from the modified rootfs
	layer, err = r.createDiff(ctx, key)
	if err != nil {
		r.logger.Errorf("failed to create diff for snapshot %q: %v", key, err)
		return layer, err
	}
	child := identity.ChainID(append(parentDiffIDs, layer.DiffID)).String()
	if err := r.snapshotter.Commit(ctx, child, key); err != nil {
		r.logger.Errorf("failed to commit snapshot %q: %v", child, err)
		return layer, err
	}
	return layer, nil
}

func NewMounterImpl() *MounterImpl {
	return &MounterImpl{}
}

type MounterImpl struct {
	mounts []mount.Mount
}

func (m *MounterImpl) Mount(target string, mounts ...mount.Mount) error {
	m.mounts = append(m.mounts, mounts...)
	return mount.All(mounts, target)
}

func (m *MounterImpl) Unmount(target string) error {
	return mount.UnmountMounts(m.mounts, target, 0)
}
