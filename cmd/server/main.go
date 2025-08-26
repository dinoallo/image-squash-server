package main

import (
	"context"
	"flag"
	"log"

	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-rebase-server/pkg/runtime"
	"github.com/lingdie/image-rebase-server/pkg/server"
)

var containerdAddress string
var namespace string

func main() {
	flag.StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	flag.StringVar(&namespace, "namespace", "default", "containerd namespace")
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
	server := server.NewServer(runtime)
	server.Run()
}
