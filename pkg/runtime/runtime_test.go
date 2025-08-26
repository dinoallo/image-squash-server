package runtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-rebase-server/pkg/options"
	"github.com/lingdie/image-rebase-server/pkg/runtime"
)

func TestRuntime_Squash(t *testing.T) {
	client, ctx, cancel, err := clientutil.NewClient(context.Background(), "default", "unix:///var/run/containerd/containerd.sock")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cancel()
	r, err := runtime.NewRuntime(client, "default")
	if err != nil {
		fmt.Println(err)
		return
	}
	opt1 := options.Option{
		SourceImage:      "docker.io/lingdie/commit:dev",
		TargetImage:      "docker.io/lingdie/commit:dev-slim",
		SquashLayerCount: 6,
	}
	if err := r.Squash(ctx, opt1); err != nil {
		fmt.Println(err)
		return
	}
}
