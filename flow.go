package teakit

import (
	"context"
	"errors"
	"go/token"
	"io"
	"reflect"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
)

// screen is what a runner shows until the user answers it.
type screen interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
	// Done reports that the screen has its answer.
	Done() bool
}

// show runs s until it is done: under AltScreen as the flow program's
// current screen, otherwise as its own inline program given opts. A nil
// error with s not done means the program ended without an answer.
func show(ctx context.Context, s screen, opts []tea.ProgramOption) error {
	if h := hostOf(ctx); h != nil {
		return h.show(ctx, s)
	}
	_, err := tea.NewProgram(inline{s}, append(opts, tea.WithContext(ctx))...).Run()
	return err
}

// interrupted reports that ctx ended as if ctrl+c was pressed: AltScreen
// caught it between screens, or its program ended before the flow did.
func interrupted(ctx context.Context) bool {
	return ctx.Err() != nil && errors.Is(context.Cause(ctx), ErrLeave)
}

// inline runs one screen as its own program. The answered screen renders
// empty, so quitting clears the frame.
type inline struct{ s screen }

func (w inline) Init() tea.Cmd { return w.s.Init() }

func (w inline) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w.s.Done() {
		return w, nil
	}
	cmd := w.s.Update(msg)
	if w.s.Done() {
		return w, tea.Batch(cmd, tea.Quit)
	}
	return w, cmd
}

func (w inline) View() tea.View {
	if w.s.Done() {
		return tea.NewView("")
	}
	return screenView(w.s.View())
}

// screenView asks for key event types, so that a held key reports IsRepeat
// and does not go on from a diff.
func screenView(content string) tea.View {
	v := tea.NewView(content)
	v.KeyboardEnhancements.ReportEventTypes = true
	return v
}

// AltScreen runs flow on the terminal's alternate screen as one Bubble Tea
// program, so the shell text before it stays hidden until flow returns, and
// returns what flow returns. Each runner flow calls with its context, such as
// Step.Run, shows its screen in that program and ignores its own program
// options; opts and out configure the program. Runners called at the same
// time take turns. The program switches screens with messages of its own,
// and the messages of a screen's commands reach it wrapped, so a
// tea.WithFilter in opts would not see what the screens receive and could
// stop the switching; filter in a screen's model instead.
//
// The program owns the terminal until flow returns, so show progress with
// Wait rather than printing; tea.Println output is dropped on the alternate
// screen. Between screens the last one stays drawn but takes no keys, since
// clearing the screen would make terminals such as iTerm2 save it to the
// scrollback. Ctrl+C between screens, or the program ending early, as on a
// termination signal, cancels flow's context with ErrLeave as its cause, and
// a runner given that context returns as if ctrl+c was pressed on its screen.
func AltScreen(ctx context.Context, out io.Writer, flow func(context.Context) error, opts ...tea.ProgramOption) error {
	flowCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	h := &flowHost{cancel: cancel, turn: make(chan struct{}, 1), ended: make(chan struct{})}
	h.p = tea.NewProgram(h, append(opts, tea.WithOutput(out), tea.WithContext(ctx))...)
	var runErr error
	go func() {
		_, err := h.p.Run()
		runErr = err
		cause := ErrLeave
		if err != nil && !errors.Is(err, tea.ErrInterrupted) {
			cause = err
		}
		cancel(cause)
		close(h.ended)
	}()
	stop := sync.OnceFunc(func() {
		// Program.Quit does nothing before Run has started; Send waits.
		h.p.Send(tea.Quit())
		<-h.ended
	})
	// Restore the terminal before a panic in flow reaches the caller.
	defer stop()
	err := flow(context.WithValue(flowCtx, hostKey{}, h))
	stop()
	if err == nil && runErr != nil && !errors.Is(runErr, tea.ErrInterrupted) {
		return runErr
	}
	return err
}

type hostKey struct{}

// hostOf returns the AltScreen program that ctx runs in, or nil.
func hostOf(ctx context.Context) *flowHost {
	h, _ := ctx.Value(hostKey{}).(*flowHost)
	return h
}

// flowHost is the AltScreen program's model. It shows one runner's screen
// at a time, and between screens the frame of the last one.
type flowHost struct {
	p      *tea.Program
	cancel context.CancelCauseFunc
	turn   chan struct{} // held by the runner whose screen is up
	ended  chan struct{} // closed once the program has stopped
	at     *attachment   // the screen that is up, or nil
	last   string        // the frame drawn last
	size   tea.Msg       // the last tea.WindowSizeMsg, for the next screen
}

// attachment is one showing of a screen.
type attachment struct {
	s    screen
	done chan struct{} // closed when s is done
	over atomic.Bool   // set once s is no longer shown
}

type attachMsg struct{ at *attachment }

// detachMsg takes down at's screen and closes ack.
type detachMsg struct {
	at  *attachment
	ack chan struct{}
}

// scoped carries a message from a command of at's screen, so that it
// reaches only that showing of the screen.
type scoped struct {
	at  *attachment
	msg tea.Msg
}

// show makes s the current screen and waits until it is done or ctx ends.
// Runners that call it at once take turns. It returns only once the
// program no longer touches s.
func (h *flowHost) show(ctx context.Context, s screen) error {
	select {
	case h.turn <- struct{}{}:
		defer func() { <-h.turn }()
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	at := &attachment{s: s, done: make(chan struct{})}
	h.p.Send(attachMsg{at})
	select {
	case <-at.done:
		return nil
	case <-ctx.Done():
	}
	ack := make(chan struct{})
	h.p.Send(detachMsg{at, ack})
	select {
	case <-ack:
	case <-h.ended:
	}
	select {
	case <-at.done:
		return nil
	default:
		return context.Cause(ctx)
	}
}

func (h *flowHost) Init() tea.Cmd { return nil }

func (h *flowHost) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case attachMsg:
		h.at = msg.at
		cmd := h.at.s.Init()
		if h.size != nil {
			cmd = tea.Batch(cmd, h.at.s.Update(h.size))
		}
		return h, h.settle(cmd)
	case detachMsg:
		msg.at.over.Store(true)
		if h.at == msg.at {
			h.at = nil
		}
		close(msg.ack)
		return h, nil
	case scoped:
		if h.at != msg.at {
			return h, nil // from a screen that is done
		}
		return h, h.settle(h.at.s.Update(msg.msg))
	case tea.WindowSizeMsg:
		h.size = msg
	case tea.KeyPressMsg:
		if h.at == nil && msg.String() == "ctrl+c" {
			h.cancel(ErrLeave)
		}
	}
	if h.at == nil {
		return h, nil
	}
	if h.size == nil {
		// Nothing is drawn before the first size, so no input but ctrl+c
		// may answer the screen yet.
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() != "ctrl+c" {
				return h, nil
			}
		case tea.PasteMsg, tea.MouseMsg:
			return h, nil
		}
	}
	return h, h.settle(h.at.s.Update(msg))
}

// settle scopes the screen's cmd to its showing, and releases the screen
// once it has its answer.
func (h *flowHost) settle(cmd tea.Cmd) tea.Cmd {
	at := h.at
	if at.s.Done() {
		at.over.Store(true)
		h.at = nil
		close(at.done)
	}
	return at.scope(cmd)
}

var cmdType = reflect.TypeFor[tea.Cmd]()

// scope tags what cmd sends with at. A batch or sequence keeps its kind,
// with each of its commands scoped. Any other message only Bubble Tea acts on
// goes to it unwrapped, in order; it is dropped if at's screen is gone by the
// time it comes back, but once sent it can act after the screen ends.
func (at *attachment) scope(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if msg == nil {
			return nil
		}
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == cmdType {
			cmds := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
			for i := range v.Len() {
				c, _ := v.Index(i).Interface().(tea.Cmd)
				cmds.Index(i).Set(reflect.ValueOf(at.scope(c)))
			}
			return cmds.Interface()
		}
		if programMsg(msg) {
			if at.over.Load() {
				return nil
			}
			return msg
		}
		return scoped{at, msg}
	}
}

// programMsg reports a message that only Bubble Tea acts on, such as the
// one tea.Println sends. A screen's tea.Quit is not one: the
// screen ends by being done, and the flow's program keeps running.
func programMsg(msg tea.Msg) bool {
	if _, raw := msg.(tea.RawMsg); raw {
		return true
	}
	t := reflect.TypeOf(msg)
	return t.PkgPath() == reflect.TypeFor[tea.Model]().PkgPath() && !token.IsExported(t.Name())
}

// View keeps the frame it draws, since a screen that is done may have
// nothing to draw. It draws nothing until the program's first size arrives,
// because Bubble Tea clears the screen when it resizes for that size.
func (h *flowHost) View() tea.View {
	if h.at != nil && h.size != nil {
		h.last = h.at.s.View()
	}
	v := screenView(h.last)
	v.AltScreen = true
	return v
}
