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
	view := func(width int) string {
		return teakit.Wrap("Looking up your repositories…", width) + "\n\nesc cancel"
	}
	return teakit.Wait(ctx, view, func(ctx context.Context) (string, error) {
		select {
		case <-time.After(3 * time.Second):
			return "4 repositories", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
}
