package teakit

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// testForm is a Form whose Init command the test controls; formDoneMsg
// ends it.
type testForm struct {
	init tea.Cmd
	done bool
}

type formDoneMsg struct{}

func (f *testForm) Init() tea.Cmd { return f.init }

func (f *testForm) Update(msg tea.Msg) tea.Cmd {
	_, f.done = msg.(formDoneMsg)
	return nil
}

func (f *testForm) View() string  { return "form" }
func (f *testForm) Done() bool    { return f.done }
func (f *testForm) AtFirst() bool { return true }

// TestStaleCommandCannotAnswerNextScreen catches an AltScreen host that
// hands a finished screen's command results to the next screen, which then
// answers without a key.
func TestStaleCommandCannotAnswerNextScreen(t *testing.T) {
	in, keys := io.Pipe()
	defer keys.Close()
	firstUp, secondUp, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	enter := func() tea.Msg {
		close(firstUp)
		<-release
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	var shown sync.Once
	head := func(int) string {
		shown.Do(func() { close(secondUp) })
		return ""
	}
	go func() {
		<-firstUp
		keys.Write([]byte("\x03"))
		<-secondUp
		close(release)
	}()
	var got error
	AltScreen(context.Background(), io.Discard, func(ctx context.Context) error {
		(&Step{Form: &testForm{init: enter}}).Run(ctx)
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		got = (&Step{List: &List{Title: "Pick", Options: []string{"a", "b"}}, Head: head}).Run(ctx)
		return nil
	}, tea.WithInput(in))
	if got != context.DeadlineExceeded {
		t.Fatalf("got %v, want the second step's deadline", got)
	}
}

// TestLeaveQuestionHoldsTheScreen catches a step that lets a message for the
// screen under the leave question end it, so Run returns nil although the
// user answered neither way.
func TestLeaveQuestionHoldsTheScreen(t *testing.T) {
	in, keys := io.Pipe()
	defer keys.Close()
	release := make(chan struct{})
	form := &testForm{init: func() tea.Msg {
		<-release
		return formDoneMsg{}
	}}
	// Release the form's message once Esc has opened the question, and
	// answer once that message has arrived.
	var opened sync.Once
	watch := func(_ tea.Model, msg tea.Msg) tea.Msg {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "esc" {
				opened.Do(func() { close(release) })
			}
		case formDoneMsg:
			go keys.Write([]byte("2"))
		}
		return msg
	}
	go keys.Write([]byte("\x1b"))
	err := (&Step{Form: form, Unsaved: true}).Run(context.Background(), tea.WithInput(in), tea.WithOutput(io.Discard), tea.WithFilter(watch))
	if err != ErrLeave {
		t.Fatalf("got %v, want ErrLeave from the leave answer", err)
	}
}

// TestFlowEndsWithoutScreen catches an AltScreen that stops its program with
// Program.Quit, which does nothing before Run has started, so a flow that
// ends before showing a screen waits until the context ends.
func TestFlowEndsWithoutScreen(t *testing.T) {
	// One thread lets the flow end before the program's goroutine starts.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	early := errors.New("invalid settings")
	err := AltScreen(ctx, io.Discard, func(context.Context) error { return early }, tea.WithInput(nil), tea.WithWindowSize(40, 10))
	if err != early || ctx.Err() != nil {
		t.Fatalf("got %v with the context at %v, want the flow's error at once", err, ctx.Err())
	}
}
