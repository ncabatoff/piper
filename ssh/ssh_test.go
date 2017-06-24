package ssh

// These tests assume that the user running them has a
// ~/.ssh/id_rsa private key whose public key is present
// in ~/.ssh/authorized_keys.

import (
	"github.com/ncabatoff/piper"
	"github.com/ncabatoff/piper/local"
	"github.com/ncabatoff/piper/test"
	"os/user"
	"path"
	"testing"
)

// Verify implementation of Executor and Launcher interfaces.
func TestInterfaces(t *testing.T) {
	_ = piper.Launcher(Launcher{})
	_ = piper.Executor(exe{})
}

func launcher(t *testing.T) *Launcher {
	user, err := user.Current()
	if err != nil {
		t.Fatalf("can't get current user: %v", err)
	}
	l, err := NewLauncher(user.Username, "127.0.0.1", path.Join(user.HomeDir, ".ssh/id_rsa"))
	if err != nil {
		t.Fatalf("Unable to create client: %v", err)
	}
	return l
}

func TestSshRunCmd(t *testing.T) {
	test.RunCmdTest(t, launcher(t))
}

func TestSshRunCmdIn(t *testing.T) {
	test.RunCmdInTest(t, launcher(t))
}

func TestSshCapture(t *testing.T) {
	test.CaptureTest(t, launcher(t))
}

func TestSshPipes(t *testing.T) {
	l := launcher(t)
	// Test ssh -> local
	test.PipeTest(t, l, local.Launcher{})
	// Test local -> ssh
	test.PipeTest(t, local.Launcher{}, l)
	// Test ssh -> ssh
	test.PipeTest(t, l, l)
}
