package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingdie/image-rebase-server/cmd/cli/cmd"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.TraceLevel)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Root.ExecuteContext(ctx); err != nil {
		logrus.WithError(err).Trace("Command execution failed")
		os.Exit(1)
	}
}
