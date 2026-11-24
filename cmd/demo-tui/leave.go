package main

import (
	"context"
	_ "embed"

	"github.com/undervoke/teakit"
)

//go:embed leave.go
var leaveSource string

func demoLeave(ctx context.Context) (string, error) {
	d, err := teakit.AskLeave(ctx, teakit.LeaveConfig{
		Title:      "Leave this screen?",
		Body:       "Unanswered input will be lost.",
		StayLabel:  "Stay",
		LeaveLabel: "Leave",
		StayKeys:   []string{"n", "q"},
		LeaveKeys:  []string{"y"},
	})
	if err != nil {
		return "", err
	}
	switch d {
	case teakit.Interrupt:
		return "interrupted", nil
	case teakit.Leave:
		return "leave", nil
	}
	return "stay", nil
}
