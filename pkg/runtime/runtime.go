package runtime

import (
	"context"
	"fmt"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/nerdctl/pkg/clientutil"
)

type Runtime struct {
	client *containerd.Client
	logger *logrus.Logger

	differ       containerd.DiffService
	imagestore   images.Store
	contentstore content.Store
	snapshotter  snapshots.Snapshotter

	runtimeCtx context.Context
	cancel     context.CancelFunc
}

func NewRuntime(ctx context.Context, options options.RootOptions) (*Runtime, error) {
	criClient, runtimeCtx, cancel, err := clientutil.NewClient(
		ctx,
		options.Namespace,
		options.ContainerdAddress,
	)
	if err != nil {
		return nil, err
	}
	// set up logger
	logger := logrus.New()
	switch options.LogLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logger.SetLevel(logrus.FatalLevel)
	case "panic":
		logger.SetLevel(logrus.PanicLevel)
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	case "":
		// do nothing, use the default logrus log level
	default:
		return nil, fmt.Errorf("unknown log level: %s", options.LogLevel)
	}
	// set up namespace
	runtimeCtx = namespaces.WithNamespace(runtimeCtx, options.Namespace)

	return &Runtime{
		client:       criClient,
		differ:       criClient.DiffService(),
		imagestore:   criClient.ImageService(),
		contentstore: criClient.ContentStore(),
		logger:       logger,
		// use default snapshotter
		snapshotter: criClient.SnapshotService(""),
		runtimeCtx:  runtimeCtx,
		cancel:      cancel,
	}, nil
}
