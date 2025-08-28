package runtime

import (
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
)

const (
	emptyDigest = digest.Digest("")
)

type Runtime struct {
	client    *containerd.Client
	namespace string
	logger    *logrus.Logger

	differ       containerd.DiffService
	imagestore   images.Store
	contentstore content.Store
	snapshotter  snapshots.Snapshotter
}

func NewRuntime(client *containerd.Client, namespace string) (*Runtime, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	return &Runtime{
		client:       client,
		namespace:    namespace,
		differ:       client.DiffService(),
		imagestore:   client.ImageService(),
		contentstore: client.ContentStore(),
		logger:       logger,
		// use default snapshotter
		snapshotter: client.SnapshotService(""),
	}, nil
}
