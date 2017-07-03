package local

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ncabatoff/piper"
)

type (
	// Launcher implements piper.Launcher by spawning a local process
	Launcher struct{}

	// exe implements piper.Executor by wrapping os/exec.Cmd
	exe struct {
		*exec.Cmd
		cancel  context.CancelFunc
		command string
	}
)

func (l Launcher) String() string {
	return "local"
}

// Errorf implements the piper.Launcher interface.
func (l Launcher) Errorf(pat string, args ...interface{}) error {
	return fmt.Errorf(pat, args)
}

// Launch implements the piper.Launcher interface by invoking sh.
func (l Launcher) Launch(cmd string) (piper.Executor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return exe{exec.CommandContext(ctx, "sh", "-c", cmd), cancel, cmd}, nil
}

// Close implements the piper.Launcher interface.
func (l Launcher) Close() error {
	return nil
}

// Errorf implements the piper.Executor interface.
func (e exe) Errorf(pat string, args ...interface{}) error {
	pfx := fmt.Sprintf("cmd {%s} :", e.command)
	return fmt.Errorf("%s: %v", pfx, fmt.Errorf(pat, args...))
}

// Command implements the piper.Launcher interface.
func (e exe) Command() string {
	return e.command
}

// Kill implements the piper.Launcher interface.
func (e exe) Kill() error {
	e.cancel()
	return nil
}
