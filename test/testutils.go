package test

import (
	"fmt"
	"github.com/ncabatoff/piper"
	"math/rand"
	"testing"
)

func RunCmdTest(t *testing.T, lch piper.Launcher) {
	err := piper.RunCmd(lch, "true")
	if err != nil {
		t.Errorf("running 'true' returned an error: %v", err)
	}
	err = piper.RunCmd(lch, "false")
	if err == nil {
		t.Errorf("running 'false' returned success")
	}
}

func RunCmdInTest(t *testing.T, lch piper.Launcher) {
	err := piper.RunCmdStrIn(lch, "bogus.command", "")
	if err == nil {
		t.Errorf("running 'bogus.command' returned success")
	}

	err = piper.RunCmdStrIn(lch, "grep -q ab", "abc")
	if err != nil {
		t.Errorf("grep -q 'ab' didn't match 'abc': %v", err)
	}
	err = piper.RunCmdStrIn(lch, "grep -q ba", "abc")
	if err == nil {
		t.Errorf("grep -q 'ba' matched 'abc'")
	}
}

// CaptureTest runs a command whose output is captured.
func CaptureTest(t *testing.T, lch piper.Launcher) {
	payload := fmt.Sprintf("%d", rand.Int31)
	stdout, stderr, err := piper.RunCmdCapture(lch, "echo -n "+payload)
	if stderr != "" || err != nil {
		t.Errorf("ssh produced errors, err=%v stderr=%q", err, stderr)
	}
	if stdout != payload {
		t.Errorf("expected %q on stdout, got %q", payload, stdout)
	}
}

// PipeTest verifies Pipe() on the given launchers
// by having the source emit something which the
// sink reads and passes through to stdout.
func PipeTest(t *testing.T, lchsrc, lchsnk piper.Launcher) {
	payload := fmt.Sprintf("%d", rand.Int31)
	src := piper.Launchable{lchsrc, "echo -n " + payload}
	snk := piper.Launchable{lchsnk, "cat"}

	pr := piper.Pipe(src, snk)
	if pr.Err != nil {
		t.Errorf("error piping: %v", pr.Err)
	}

	if pr.SnkStdout != payload {
		t.Errorf("expected %q, got %q", payload, pr.SnkStdout)
	}
}
