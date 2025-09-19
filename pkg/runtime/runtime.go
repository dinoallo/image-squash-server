package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/timer"
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/nerdctl/pkg/clientutil"
)

type Runtime struct {
	client *containerd.Client
	*logrus.Logger
	timer.Timer

	differ          containerd.DiffService
	imagestore      images.Store
	contentstore    content.Store
	snapshotter     snapshots.Snapshotter
	snapshotterName string

	runtimeCtx context.Context
	cancel     context.CancelFunc
	leaseDone  func(context.Context) error
}

func NewRuntime(ctx context.Context, options options.RootOptions) (*Runtime, error) {
	criClient, runtimeCtx, cancel, err := clientutil.NewClient(
		ctx,
		options.Namespace,
		options.ContainerdAddress,
	)
	if err != nil {
		cancel()
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
	// Don't gc me and clean the dirty data after 24 hour! (or the temp snapshot may be gced when we are debugging)
	runtimeCtx, done, err := criClient.WithLease(runtimeCtx, leases.WithRandomID(), leases.WithExpiration(24*time.Hour))
	if err != nil {
		cancel()
		return nil, err
	}
	t, err := timer.NewTimerImpl(logger)
	if err != nil {
		cancel()
		return nil, err
	}
	snapshotterName, err := resolveSnapshotterName(runtimeCtx, criClient, "")
	if err != nil {
		cancel()
		return nil, err
	}
	return &Runtime{
		client:       criClient,
		differ:       criClient.DiffService(),
		imagestore:   criClient.ImageService(),
		contentstore: criClient.ContentStore(),
		Logger:       logger,
		// use default snapshotter
		snapshotter:     criClient.SnapshotService(snapshotterName),
		snapshotterName: snapshotterName,
		runtimeCtx:      runtimeCtx,
		cancel:          cancel,
		leaseDone:       done,
		Timer:           t,
	}, nil
}

func (r *Runtime) Close() error {
	// release the lease
	if err := r.leaseDone(r.runtimeCtx); err != nil {
		return err
	}
	r.cancel()
	// close the client
	if err := r.client.Close(); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) Context() context.Context {
	return r.runtimeCtx
}

const (
	DefaultSnapshotter = "overlayfs"
)

func resolveSnapshotterName(ctx context.Context, c *containerd.Client, name string) (string, error) {
	if name == "" {
		label, err := c.GetLabel(ctx, defaults.DefaultSnapshotterNSLabel)
		if err != nil {
			return "", err
		}

		if label != "" {
			name = label
		} else {
			name = DefaultSnapshotter
		}
	}

	return name, nil
}
