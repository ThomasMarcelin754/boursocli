package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/thomasmarcelin754/boursobank/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// cobra reads ctx via cmd.Context(); set it on the root execution.
	if err := cli.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
