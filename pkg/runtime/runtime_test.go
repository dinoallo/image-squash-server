package runtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
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
	opt1 := options.RebaseOption{
		OriginalImage: "docker.io/lingdie/commit:dev",
		NewImage:      "docker.io/lingdie/commit:dev-slim",
		BaseImage:     "docker.io/library/debian:bookworm-slim",
	}
	if err := r.Rebase(ctx, opt1); err != nil {
		fmt.Println(err)
		return
	}
}
