package teakit

import (
	"context"
	"errors"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// testScreen is a screen with a command of its own, which any key ends.
type testScreen struct {
	init tea.Cmd
	done bool
}

func (s *testScreen) Init() tea.Cmd { return s.init }

func (s *testScreen) Update(msg tea.Msg) tea.Cmd {
	_, s.done = msg.(tea.KeyPressMsg)
	return nil
}

func (s *testScreen) View() string { return "screen" }
func (s *testScreen) Done() bool   { return s.done }

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
		show(ctx, &testScreen{init: enter}, nil)
		ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		got = (&Step{Select: &Select{Title: "Pick", Options: []string{"a", "b"}}, Head: head}).Run(ctx)
		return nil
	}, tea.WithInput(in))
	if got != context.DeadlineExceeded {
		t.Fatalf("got %v, want the second step's deadline", got)
	}
}

// screenLog records a program's output, and reports when each text it was
// made for first appears in it.
type screenLog struct {
	mu    sync.Mutex
	out   strings.Builder
	shown map[string]chan struct{}
}

func newScreenLog(texts ...string) *screenLog {
	l := &screenLog{shown: map[string]chan struct{}{}}
	for _, text := range texts {
		l.shown[text] = make(chan struct{})
	}
	return l
}

func (l *screenLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out.Write(p)
	for text, c := range l.shown {
		select {
		case <-c:
		default:
			if strings.Contains(l.out.String(), text) {
				close(c)
			}
		}
	}
	return len(p), nil
}

func (l *screenLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.String()
}

var termToken = regexp.MustCompile(`\x1b\[([?<>=]?)([0-9;]*)([@-~])|\x1b[^\[]|[\s\S]`)

// savedErases counts the erases in out that iTerm2 saves to the scrollback
// on the alternate screen once something is drawn: of the whole display, or
// of the display below the cursor at the top-left corner.
func savedErases(out string) int {
	row, col, n, drawn := 0, 0, 0, false
	for _, m := range termToken.FindAllStringSubmatch(out, -1) {
		if m[3] == "" {
			switch {
			case m[0] == "\r":
				col = 0
			case m[0] == "\n":
				row++
			case m[0][0] != '\x1b':
				drawn = drawn || m[0] != " "
				col++
			}
			continue
		}
		if m[1] != "" {
			continue
		}
		p := []int{0, 0}
		for i, arg := range strings.SplitN(m[2], ";", 2) {
			p[i], _ = strconv.Atoi(arg)
		}
		switch m[3] {
		case "H":
			row, col = max(p[0]-1, 0), max(p[1]-1, 0)
		case "d":
			row = max(p[0]-1, 0)
		case "G":
			col = max(p[0]-1, 0)
		case "A":
			row -= max(p[0], 1)
		case "B":
			row += max(p[0], 1)
		case "C":
			col += max(p[0], 1)
		case "D":
			col -= max(p[0], 1)
		case "J":
			if p[0] == 2 || p[0] == 0 && row == 0 && col == 0 {
				if drawn {
					n++
				}
				drawn = false
			}
		}
	}
	return n
}

// TestFlowKeepsTheFrame catches an AltScreen flow that erases a drawn
// screen, which iTerm2 then keeps in the scrollback above the shell: by
// clearing between screens or after the last one, by drawing before the
// program's first size, whose resize clears the screen, or on a Bubble Tea
// whose renderer erases trailing rows from the row above them.
func TestFlowKeepsTheFrame(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	log := newScreenLog("HEADER", "next")
	// Hold the program's first size until the first screen is drawn, or
	// for long enough that it would have been.
	held := false
	hold := func(m tea.Model, msg tea.Msg) tea.Msg {
		if size, ok := msg.(tea.WindowSizeMsg); ok && !held {
			held = true
			go func() {
				select {
				case <-log.shown["HEADER"]:
				case <-time.After(100 * time.Millisecond):
				}
				m.(*flowHost).p.Send(size)
			}()
			return nil
		}
		return msg
	}
	show := func(ctx context.Context, cfg WaitConfig) error {
		_, err := Wait(ctx, cfg, func(ctx context.Context) (struct{}, error) {
			select {
			case <-log.shown[cfg.Title]:
			case <-ctx.Done():
			}
			return struct{}{}, ctx.Err()
		})
		return err
	}
	err := AltScreen(ctx, log, func(ctx context.Context) error {
		if err := show(ctx, WaitConfig{Title: "HEADER", Hint: "detail"}); err != nil {
			return err
		}
		// Long enough for the renderer to draw what shows between screens.
		time.Sleep(50 * time.Millisecond)
		return show(ctx, WaitConfig{Title: "next"})
	}, tea.WithInput(nil), tea.WithWindowSize(40, 10), tea.WithFilter(hold))
	if err != nil {
		t.Fatalf("got %v, want both screens shown", err)
	}
	if n := savedErases(log.String()); n != 0 {
		t.Fatalf("a drawn screen was erased %d times, want never: %q", n, log.String())
	}
}

// TestFlowStaysAtLoneLeaveQuestion catches an AltScreen host that lets a
// key answer a screen before the program's first size lets it be drawn, or
// that draws a screen once it is done: a Step without a question or a Form has
// nothing to draw after the user stays, and drawing it panics.
func TestFlowStaysAtLoneLeaveQuestion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	in, keys := io.Pipe()
	defer keys.Close()
	log := newScreenLog("Stay")
	// Hold the program's first size until "2", which would leave, has
	// reached the question; then stay once the question is drawn. "2" goes
	// once both the question and the held size are in, in either order.
	var size tea.Msg
	attached := false
	press := func() {
		if size != nil && attached {
			go keys.Write([]byte("2"))
		}
	}
	hold := func(m tea.Model, msg tea.Msg) tea.Msg {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			if size == nil {
				size = msg
				press()
				return nil
			}
		case attachMsg:
			attached = true
			press()
		case tea.KeyPressMsg:
			if msg.String() == "2" {
				go m.(*flowHost).p.Send(size)
			}
		}
		return msg
	}
	go func() {
		select {
		case <-log.shown["Stay"]:
			keys.Write([]byte("\r"))
		case <-ctx.Done():
		}
	}()
	stayed := ErrLeave
	err := AltScreen(ctx, log, func(ctx context.Context) error {
		stayed = (&Step{}).Run(ctx)
		return nil
	}, tea.WithInput(in), tea.WithWindowSize(80, 24), tea.WithFilter(hold))
	if err != nil || stayed != nil {
		t.Fatalf("got %v from the flow and %v from the step, want nil from both", err, stayed)
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
