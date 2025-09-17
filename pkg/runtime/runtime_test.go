package runtime_test

import (
	"context"
	"testing"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
)

func TestRuntime_Rebase(t *testing.T) {
	rootOpts := options.RootOptions{
		ContainerdAddress: "unix:///var/run/containerd/containerd.sock",
		Namespace:         "k8s.io",
	}
	r, err := runtime.NewRuntime(context.TODO(), rootOpts)
	if err != nil {
		t.Errorf("NewRuntime() error = %v", err)
		return
	}
	rebaseOpts := options.RebaseOptions{
		RootOptions:   rootOpts,
		OriginalImage: "docker.io/lingdie/commit:dev",
		NewImage:      "docker.io/lingdie/commit:dev-slim",
		BaseImage:     "docker.io/library/debian:bookworm-slim",
		AutoSquash:    true,
	}
	if err := r.Rebase(r.Context(), rebaseOpts); err != nil {
		t.Errorf("Rebase() error = %v", err)
		return
	}
}
