//go:build unix

package teakit

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestRunnersTreatTerminationAsCtrlC catches a runner that returns its
// model's result without checking that the user answered. Bubble Tea ends a
// program gracefully on SIGTERM, so the step would read as an option picked
// and Wait as finished work.
//
// Reproduced on v0.6.0 in a real terminal: kill -TERM made Step.Run return nil.
func TestRunnersTreatTerminationAsCtrlC(t *testing.T) {
	// Keep SIGTERM from killing the test binary while no program listens.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)
	tests := []struct {
		name string
		run  func(opts []tea.ProgramOption) (got, want any)
	}{
		{"Step", func(opts []tea.ProgramOption) (any, any) {
			s := &Step{Select: &Select{Title: "Pick", Options: []string{"a", "b"}}}
			return s.Run(context.Background(), opts...), ErrLeave
		}},
		{"Wait", func(opts []tea.ProgramOption) (any, any) {
			_, err := Wait(context.Background(), func(int) string { return "working" }, func(ctx context.Context) (int, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			}, opts...)
			return err, ErrLeave
		}},
		{"AskLeave", func(opts []tea.ProgramOption) (any, any) {
			d, _ := AskLeave(context.Background(), LeaveConfig{}, opts...)
			return d, Interrupt
		}},
		{"ReviewDiff", func(opts []tea.ProgramOption) (any, any) {
			d, _ := ReviewDiff(context.Background(), DiffConfig{}, nil, opts...)
			return d, DiffInterrupt
		}},
		{"AltScreen", func(opts []tea.ProgramOption) (any, any) {
			var err error
			AltScreen(context.Background(), io.Discard, func(ctx context.Context) error {
				err = (&Step{Select: &Select{Title: "Pick", Options: []string{"a", "b"}}}).Run(ctx)
				return nil
			}, opts...)
			return err, ErrLeave
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, w := io.Pipe()
			defer w.Close()
			// Repeat the signal, since the program installs its handler
			// asynchronously after Run starts.
			stop := make(chan struct{})
			go func() {
				for {
					select {
					case <-stop:
						return
					case <-time.After(20 * time.Millisecond):
						syscall.Kill(os.Getpid(), syscall.SIGTERM)
					}
				}
			}()
			got, want := tt.run([]tea.ProgramOption{tea.WithInput(in), tea.WithOutput(io.Discard)})
			close(stop)
			if got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}
