package main

import (
	"context"
	"flag"
	"log"

	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-rebase-server/pkg/options"
	"github.com/lingdie/image-rebase-server/pkg/runtime"
)

var containerdAddress string
var namespace string
var sourceImageRef string
var targetImageRef string
var squashLayerCount int

func main() {
	flag.StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	flag.StringVar(&namespace, "namespace", "k8s.io", "containerd namespace")
	flag.StringVar(&sourceImageRef, "source-image", "", "source image reference")
	flag.StringVar(&targetImageRef, "target-image", "", "target image reference")
	flag.IntVar(&squashLayerCount, "squash-layer-count", 2, "squash layer count")
	flag.Parse()
	containerdClient, _, _, err := clientutil.NewClient(
		context.Background(),
		namespace,
		containerdAddress,
	)
	if err != nil {
		log.Fatalf("Failed to create containerd client: %v", err)
	}

	runtime, err := runtime.NewRuntime(
		containerdClient,
		namespace,
	)
	if err != nil {
		log.Fatalf("Failed to create runtime: %v", err)
	}

	err = runtime.Squash(context.Background(), options.Option{
		SourceImage:      sourceImageRef,
		TargetImage:      targetImageRef,
		SquashLayerCount: squashLayerCount,
	})
	if err != nil {
		log.Fatalf("Failed to squash image: %v", err)
	}

}
