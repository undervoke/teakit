// Command leave runs a generic leave question as an inline step.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/undervoke/teakit"
)

func main() {
	d, err := teakit.AskLeave(context.Background(), teakit.LeaveConfig{
		Title:      "Leave this screen?",
		Body:       "Unanswered input will be lost.",
		StayLabel:  "Stay",
		LeaveLabel: "Leave",
		StayKeys:   []string{"n", "q"},
		LeaveKeys:  []string{"y"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "leave:", err)
		os.Exit(1)
	}
	switch d {
	case teakit.Interrupt:
		fmt.Println("interrupted; leaving")
		os.Exit(130)
	case teakit.Leave:
		fmt.Println("leaving")
	default:
		fmt.Println("staying")
	}
}
