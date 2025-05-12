package terminal

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"io"
	"os"
	"time"
)

func Associate(ctx context.Context, session *ssh.Session) error {
	// Configure interactive terminal.
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	width, height, err := GetTerminalSize()
	if err != nil {
		width, height = 80, 24
	}

	if err := session.RequestPty("xterm", height, width, modes); err != nil {
		return err
	}

	stop, err := StartResizeWatcher(session)
	if err != nil {
		return err
	}
	defer stop()

	// Switch terminal to raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %w", err)
	}
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}()

	// Start interactive shell.
	if err := session.Shell(); err != nil {
		return err
	}

	sessionChan := make(chan error)

	// Wait for finish.
	go func() {
		err = session.Wait()
		if err != nil && err != io.EOF {
			sessionChan <- err
		}
		sessionChan <- nil
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-sessionChan:
		return err
	}
}

func StartResizeWatcher(session *ssh.Session) (func(), error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("stdin is not a terminal")
	}

	width0, height0, _ := term.GetSize(int(os.Stdin.Fd()))

	ticker := time.NewTicker(500 * time.Millisecond)

	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				width, height, err := GetTerminalSize()
				if err != nil {
					continue
				}
				// Resize window if dimensions have changed.
				if width != width0 || height != height0 {
					_ = session.WindowChange(height, width)
					width0, height0 = width, height
				}
			}
		}
	}()

	stop := func() {
		done <- struct{}{}
	}

	return stop, nil
}
