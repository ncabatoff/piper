package local

import (
	"github.com/ncabatoff/piper"
	"github.com/ncabatoff/piper/test"
	"testing"
)

// Verify implementation of Executor and Launcher interfaces.
func TestInterfaces(t *testing.T) {
	_ = piper.Launcher(Launcher{})
	_ = piper.Executor(exe{})
}

func TestLocalRunCmd(t *testing.T) {
	test.RunCmdTest(t, Launcher{})
}

func TestLocalRunCmdIn(t *testing.T) {
	test.RunCmdInTest(t, Launcher{})
}

func TestLocalCapture(t *testing.T) {
	test.CaptureTest(t, Launcher{})
}

func TestLocalPipe(t *testing.T) {
	test.PipeTest(t, Launcher{}, Launcher{})
}
