package terminal

import (
	"context"
	"io"
	"sync"

	"github.com/UserExistsError/conpty"
)

type Terminal struct {
	cpty   *conpty.ConPty
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func New(shell string, cols, rows int, workDir string) (*Terminal, error) {
	opts := []conpty.ConPtyOption{
		conpty.ConPtyDimensions(cols, rows),
	}
	if workDir != "" {
		opts = append(opts, conpty.ConPtyWorkDir(workDir))
	}
	cpty, err := conpty.Start(shell, opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &Terminal{
		cpty:   cpty,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(t.done)
		cpty.Wait(ctx)
	}()

	return t, nil
}

func (t *Terminal) Read(p []byte) (int, error) {
	return t.cpty.Read(p)
}

func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cpty.Write(p)
}

func (t *Terminal) Resize(cols, rows int) error {
	return t.cpty.Resize(cols, rows)
}

func (t *Terminal) Done() <-chan struct{} {
	return t.done
}

func (t *Terminal) Close() error {
	t.cancel()
	return t.cpty.Close()
}

func (t *Terminal) Pipe(w io.Writer) error {
	buf := make([]byte, 8192)
	for {
		n, err := t.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}
