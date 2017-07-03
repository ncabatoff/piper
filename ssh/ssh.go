package ssh

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"

	"github.com/ncabatoff/piper"
	"golang.org/x/crypto/ssh"
)

const defaultSshPort = 22

type (
	// exe implements piper.Executor
	exe struct {
		*ssh.Session
		launchdesc string
		command    string
	}

	// Launcher implements piper.Launcher
	Launcher struct {
		*ssh.Client
	}

	readerDummyCloser struct {
		io.Reader
	}
)

func (r readerDummyCloser) Close() error {
	return nil
}

// NewClient creates an ssh client.
func NewClient(hostname string, port int, cfg ssh.ClientConfig) (*ssh.Client, error) {
	hostport := net.JoinHostPort(hostname, fmt.Sprintf("%d", port))
	cli, err := ssh.Dial("tcp", hostport, &cfg)
	if err != nil {
		target := fmt.Sprintf("%s@%s", cfg.User, hostport)
		return nil, fmt.Errorf("error opening ssh connection to %s: %v", target, err)
	}
	return cli, nil
}

// NewConfig creates an ssh client config that ignores insecure host keys.
// keyfname is the path to a private ssh key file to use.
func NewConfig(user, keyfname string) (*ssh.ClientConfig, error) {
	p, err := ioutil.ReadFile(keyfname)
	if err != nil {
		return nil, err
	}
	s, err := ssh.ParsePrivateKey(p)
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

// String implements the piper.Launcher interface.
func (l Launcher) String() string {
	user, hostport := l.Client.Conn.User(), l.Client.Conn.RemoteAddr()
	return fmt.Sprintf("%s@%s", user, hostport)
}

// Close implements the piper.Launcher interface.
func (l Launcher) Close() error {
	return l.Client.Close()
}

// NewLauncher creates a new Launcher by starting an ssh client.
// It will ignores insecure host keys.
// keyfname is the path to a private ssh key file to use.
func NewLauncher(user, host, keyfname string) (*Launcher, error) {
	cfg, err := NewConfig(user, keyfname)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure ssh client: %v", err)
	}

	client, err := NewClient(host, defaultSshPort, *cfg)
	if err != nil {
		return nil, err
	}
	return &Launcher{Client: client}, nil
}

// Launch implements the piper.Launcher interface by creating a new ssh session.
func (l Launcher) Launch(command string) (piper.Executor, error) {
	sess, err := l.Client.NewSession()
	if err != nil {
		return nil, err
	}

	return &exe{launchdesc: l.String(), Session: sess, command: command}, nil
}

// Errorf implements the piper.Launcher interface.
func (l Launcher) Errorf(pat string, args ...interface{}) error {
	pfx := fmt.Sprintf("%s: ", l)
	return fmt.Errorf("%s: %v", pfx, fmt.Errorf(pat, args...))
}

// Errorf implements the piper.Executor interface.
func (e exe) Errorf(pat string, args ...interface{}) error {
	pfx := fmt.Sprintf("cmd %s{%s} :", e.launchdesc, e.command)
	return fmt.Errorf("%s: %v", pfx, fmt.Errorf(pat, args...))
}

// Command implements the piper.Executor interface.
func (e exe) Command() string {
	return e.command
}

// Run implements the piper.Executor interface.
func (e exe) Run() error {
	defer e.Session.Close()
	return e.Session.Run(e.command)
}

// Start implements the piper.Executor interface.
func (e exe) Start() error {
	return e.Session.Start(e.command)
}

// Wait implements the piper.Executor interface.
func (e exe) Wait() error {
	defer e.Session.Close()
	return e.Session.Wait()
}

// Kill implements the piper.Executor interface.
func (e exe) Kill() error {
	return e.Session.Signal(ssh.SIGKILL)
}

// StderrPipe implements the piper.Executor interface.
func (e exe) StderrPipe() (io.ReadCloser, error) {
	r, err := e.Session.StderrPipe()
	if err != nil {
		return nil, err
	}
	return readerDummyCloser{r}, nil
}

// StdoutPipe implements the piper.Executor interface.
func (e exe) StdoutPipe() (io.ReadCloser, error) {
	r, err := e.Session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return readerDummyCloser{r}, nil
}
