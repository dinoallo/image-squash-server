package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/util"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r *Runtime) Remove(ctx context.Context, opt options.RemoveOption) error {
	//TODO: implement me
	// get the original image
	r.logger.Infof("start to remove file %q from image %q", opt.File, opt.OriginalImage)
	origImage, err := r.GetImage(ctx, opt.OriginalImage)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.OriginalImage, err)
		return err
	}
	var (
		newLayers  []ocispec.Descriptor
		newDiffIDs []digest.Digest
	)
	newLayer, newDiffID, err := r.createRemovalLayer(ctx, origImage.Config, opt.File)
	if err != nil {
		r.logger.Errorf("failed to create removal layer for file %q: %v", opt.File, err)
		return err
	}
	newLayers = append(newLayers, newLayer)
	newDiffIDs = append(newDiffIDs, newDiffID)
	newImageConfig, err := r.GenerateImageConfig(ctx, origImage.Image, origImage.Config, newDiffIDs)
	if err != nil {
		r.logger.Errorf("failed to generate new image config for %q: %v", opt.NewImage, err)
		return err
	}
	commitManifestDesc, _, err := r.WriteContentsForImage(ctx, "overlayfs", newImageConfig, origImage.Manifest.Layers, newLayers)
	if err != nil {
		r.logger.Errorf("failed to write contents for image %q: %v", opt.NewImage, err)
		return err
	}
	nImg := images.Image{
		Name:      opt.NewImage,
		Target:    commitManifestDesc,
		UpdatedAt: time.Now(),
	}
	_, err = r.UpdateImage(ctx, nImg)
	if err != nil {
		r.logger.Errorf("failed to update image %q: %v", opt.NewImage, err)
		return err
	}
	cimg := containerd.NewImage(r.client, nImg)
	// unpack image to the snapshot storage
	if err := cimg.Unpack(ctx, "overlayfs"); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", opt.NewImage, err)
		return err
	}
	return nil
}

func (r *Runtime) createRemovalLayer(ctx context.Context, origImage ocispec.Image, file string) (ocispec.Descriptor, digest.Digest, error) {
	var (
		key           = fmt.Sprintf("file-removal-%s", util.UniquePart())
		parentDiffIDs = origImage.RootFS.DiffIDs
		parent        = identity.ChainID(origImage.RootFS.DiffIDs)
	)
	// create mount target to mount the rootfs
	mountTarget, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "remove-file-")
	if err != nil {
		r.logger.Errorf("failed to create mount target %q: %v", mountTarget, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	defer os.RemoveAll(mountTarget)
	// prepare a temporary rootfs
	mounts, err := r.snapshotter.Prepare(ctx, key, parent.String())
	if err != nil {
		r.logger.Errorf("failed to prepare snapshot %q: %v", key, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	mounter := NewMounterImpl()
	if err := mounter.Mount(mountTarget, mounts...); err != nil {
		r.logger.Errorf("failed to mount rootfs %q: %v", mountTarget, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	defer mounter.Unmount(mountTarget)
	// remove the file
	if err := os.RemoveAll(filepath.Join(mountTarget, file)); err != nil {
		r.logger.Errorf("failed to remove file %q: %v", file, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	// create a diff from the modified rootfs
	newLayer, diffID, err := r.createDiff(ctx, key)
	if err != nil {
		r.logger.Errorf("failed to create diff for snapshot %q: %v", key, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	child := identity.ChainID(append(parentDiffIDs, diffID)).String()
	if err := r.snapshotter.Commit(ctx, child, key); err != nil {
		r.logger.Errorf("failed to commit snapshot %q: %v", child, err)
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	return newLayer, diffID, nil
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
