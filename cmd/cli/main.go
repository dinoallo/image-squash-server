package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingdie/image-manip-server/cmd/cli/cmd"
)

func main() {
	// logrus.SetLevel(logrus.TraceLevel)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
