package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"insight-lab/internal/app"
	"insight-lab/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Main(ctx, os.Args[1:], os.Stdout, os.Stderr, app.Run)
	stop()
	os.Exit(code)
}
