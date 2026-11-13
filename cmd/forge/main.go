package main

import (
	"context"
	"os"
	"os/signal"
)

func main() {
	// Context interrupt here for the future, when we're using Forge for GitHub or AWS API calls
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		stop()
		os.Exit(1)
	}
}
