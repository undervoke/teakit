package main

import (
	"context"
	_ "embed"
	"time"

	"github.com/undervoke/teakit"
)

//go:embed wait.go
var waitSource string

func demoWait(ctx context.Context) (string, error) {
	cfg := teakit.WaitConfig{
		Title: "Looking up your repositories…",
		Hint:  "esc cancel",
	}
	return teakit.Wait(ctx, cfg, func(ctx context.Context) (string, error) {
		select {
		case <-time.After(3 * time.Second):
			return "4 repositories", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
}
