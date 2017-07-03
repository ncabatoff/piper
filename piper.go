package piper

import (
	"bytes"
	"fmt"
	"io"
)

type (
	// Launcher knows how to create an Executor.
	Launcher interface {
		// Create an executor that can run cmd.  Launch doesn't
		// actually spawn the process, it just provides the means to.
		// That doesn't mean there are no side-effects; that depends on
		// the implementation.
		Launch(cmd string) (Executor, error)
		fmt.Stringer
		// Errorf returns an error as fmt.Errorf would, prepending a
		// description of the launcher.
		Errorf(pat string, args ...interface{}) error
		Close() error
	}

	// Launchable is a convenient way to pass around a Launcher along with
	// a specific cmd to Launch.
	Launchable struct {
		Launcher
		Cmd string
	}

	// An Executor manages a child process.  This is a lowest common
	// denominator interface of os/exec.Cmd and golang.org/x/crypto/ssh.Session.
	Executor interface {
		// Command returns the text of the command to be executed.
		Command() string
		// Errorf returns an error as fmt.Errorf would, prepending a description
		// of the command and launcher involved.
		Errorf(pat string, args ...interface{}) error
		// Run executes the command with no input and discards stdout/stderr.
		// It is the equivalent of calling Start(), then Wait().
		Run() error
		// Start spawns the command, returning an error if it can't.  Any piping
		// should be setup first.
		Start() error
		// StderrPipe returns a reader which yields what the process writes to stderr.
		StderrPipe() (io.ReadCloser, error)
		// StdinPipe returns a writer which will send what it receives
		// to the process's stdin.
		StdinPipe() (io.WriteCloser, error)
		// StderrPipe returns a reader which yields what the process writes to stdout.
		StdoutPipe() (io.ReadCloser, error)
		// Wait() waits for the command to complete and returns the result.
		// An error returns if the command is killed, or returns a non-zero exit code.
		// Failing to Wait() will result in resource leaks.  There is no need to close
		// stdout/stderr pipes provided Wait() is called.  Wait() should not be
		// called until all output pipes have been fully consumed.
		Wait() error
		// Kill terminates a command that has been Start()ed.  It returns an error if it
		// encountered one while doing so.  There is no hard guarantee that if it has
		// returned success the process has actually been called.  You still need to
		// Wait() after a Kill() to avoid resource leaks.
		Kill() error
	}

	// Verbose() wraps an existing launcher to describe what it does, and what
	// any Executor it builds does.  This includes Run, Start, Wait, and Kill
	// activities.
	Verbose struct {
		Launcher
		Logf func(format string, args ...interface{})
	}

	// verbexe is a wrapping executor that logs what it's doing.
	verbexe struct {
		Executor
		Verbose
	}
)

// LaunchCmd creates an Executor from the embedded Launcher and cmd.
func (l Launchable) LaunchCmd() (Executor, error) {
	return l.Launcher.Launch(l.Cmd)
}

// Launch implements Launcher.
func (v Verbose) Launch(cmd string) (Executor, error) {
	exe, err := v.Launcher.Launch(cmd)
	if err != nil {
		return nil, err
	}
	return verbexe{exe, v}, nil
}

// Close implements Launcher.
func (v Verbose) Close() error {
	err := v.Launcher.Close()
	v.Logf("[%s] Close returned %v", v.Launcher.String(), err)
	return err
}

// Run implements Executor.
func (ve verbexe) Run() error {
	ve.Logf("[%s] Running command [%s]", ve.Verbose.Launcher.String(), ve.Command())
	err := ve.Executor.Run()
	if err != nil {
		ve.Logf("[%s] Command [%s] failed: %v", ve.Verbose.Launcher.String(), ve.Command(), err)
	}
	return err
}

// Start implements Executor.
func (ve verbexe) Start() error {
	ve.Logf("[%s] Starting command [%s]", ve.Verbose.Launcher.String(), ve.Command())
	return ve.Executor.Start()
}

// Wait implements Executor.
func (ve verbexe) Wait() error {
	ve.Logf("[%s] Waiting for command [%s]", ve.Verbose.Launcher.String(), ve.Command())
	err := ve.Executor.Wait()
	if err != nil {
		ve.Logf("[%s] Command [%s] failed: %v", ve.Verbose.Launcher.String(), ve.Command(), err)
	}
	return err
}

// Kill implements Executor.
func (ve verbexe) Kill() error {
	ve.Logf("[%s] Killing command [%s]", ve.Verbose.Launcher.String(), ve.Command())
	return ve.Executor.Kill()
}

type (
	harness struct {
		exe    Executor
		errs   chan error
		stdin  *string
		stdout *string
		stderr *string
	}
)

func newHarness(exe Executor) *harness {
	return &harness{exe: exe, errs: make(chan error)}
}

func startCmd(lch Launcher, cmd string) (*harness, error) {
	exe, err := lch.Launch(cmd)
	if err != nil {
		return nil, lch.Errorf("error starting %s: %v", cmd, err)
	}

	return newHarness(exe), nil
}

// RunCmd executes cmd using lch, discarding any output.
func RunCmd(lch Launcher, cmd string) error {
	h, err := startCmd(lch, cmd)
	if err != nil {
		return err
	}
	return h.run()
}

func RunCmdStrIn(lch Launcher, cmd, stdin string) error {
	h, err := startCmd(lch, cmd)
	if err != nil {
		return err
	}
	h.stdin = &stdin
	return h.run()
}

// Capture executes exe and returns the stdout and stderr output it produces.
func RunCmdCapture(lch Launcher, cmd string) (stdout string, stderr string, err error) {
	h, err := startCmd(lch, cmd)
	if err != nil {
		return "", "", err
	}
	h.stdout, h.stderr = &stdout, &stderr
	err = h.run()
	return
}

func RunCmdStrInCapture(lch Launcher, cmd, stdin string) (stdout string, stderr string, err error) {
	h, err := startCmd(lch, cmd)
	if err != nil {
		return "", "", err
	}
	h.stdin, h.stdout, h.stderr = &stdin, &stdout, &stderr
	err = h.run()
	return
}

// copyClose is a helper method to write rc to w.  Once rc is exhausted or a write
// error occurs, a nil or an error is written to errs.
func copyClose(w io.Writer, rc io.Reader, errs chan error) {
	_, err := io.Copy(w, rc)
	errs <- err
}

func copyCloseStr(dest *string, rc io.Reader, errs chan error) {
	var w bytes.Buffer
	_, err := io.Copy(&w, rc)
	*dest = w.String()
	errs <- err
}

// CaptureIn executes exe, writing the string stdin to the exe's standard input.
// Returns the stdout and stderr output produced.
func (h harness) run() error {
	var errchan = make(chan error)
	// The size of errs determines how many reads we'll do from errchan.
	var errs = make([]error, 0, 3)

	var pstdout, pstderr io.ReadCloser
	var err error

	if h.stdout != nil {
		pstdout, err = h.exe.StdoutPipe()
		if err != nil {
			return h.exe.Errorf("error opening stdout pipe: %v", err)
		}
		errs = errs[:len(errs)+1]
		go func() {
			copyCloseStr(h.stdout, pstdout, errchan)
		}()
	}
	if h.stderr != nil {
		pstderr, err = h.exe.StderrPipe()
		if err != nil {
			if pstdout != nil {
				pstdout.Close()
			}
			return h.exe.Errorf("error opening stderr pipe: %v", err)
		}
		errs = errs[:len(errs)+1]
		go func() {
			copyCloseStr(h.stderr, pstderr, errchan)
		}()
	}

	if h.stdin != nil {
		pstdin, err := h.exe.StdinPipe()
		if err != nil {
			if pstdout != nil {
				pstdout.Close()
			}
			if pstderr != nil {
				pstderr.Close()
			}
			return h.exe.Errorf("error opening stdin pipe: %v", err)
		}
		go func() {
			copyClose(pstdin, bytes.NewBuffer([]byte(*h.stdin)), errchan)
			pstdin.Close()
		}()
		errs = errs[:len(errs)+1]
	}

	// The expectation is that Start() will close open fds
	// associated with the exe (e.g. from StdoutPipe) if it
	// returns an error.
	if err := h.exe.Start(); err != nil {
		return h.exe.Errorf("error starting: %v", err)
	}

	// Ok, we have a running exe now.  Capture stdout and stderr, and collect
	// all the errors from handling stdout, stderr, and possibly stdin.
	for i := range errs {
		errs[i] = <-errchan
	}

	// Now we wait for the process to exit.  Since all the errs from errchan
	// are the result of operations between a buffer and the exe, it's very
	// unlikely (impossible?) that we get errors on them without getting an
	// error from Wait().  But just in case, we'll handle that possibility.
	err = h.exe.Wait()
	if err != nil {
		err = h.exe.Errorf("completed with error: %v", err)
	} else {
		for _, e := range errs {
			if e != nil {
				err = e
				break
			}
		}
	}

	return err
}

type (
	// source is the writing side of a pipe.
	source struct {
		exe Executor
		// stdout emits what exe writes to its stdout.
		stdout io.Reader
		// stderr stores what exe writes to its stderr.
		stderr bytes.Buffer
		// errchan is written to once exe's stderr is closed: an error on
		// failure, nil on success.
		errchan <-chan error
	}

	// source is the reading side of a pipe.
	sink struct {
		exe Executor
		// stdin is fed into exe's stdin.
		stdin io.WriteCloser
		// stdout stores what exe writes to its stdout.
		stdout bytes.Buffer
		// stderr stores what exe writes to its stderr.
		stderr bytes.Buffer
		// errchan is written to once exe's stderr is closed: an error on
		// failure, nil on success.  Same goes for stdout.
		errchan <-chan error
	}

	pipe struct {
		src *source
		snk *sink
	}

	// PipeResult summarizes the result of a pipe by giving the stderr of the source,
	// the stdout and stderr of the sink, and an error describing the outcome.
	// (There's no stdout for the source because that was fed into the sink.)
	PipeResult struct {
		SrcStderr string
		SnkStderr string
		SnkStdout string
		Err       error
	}
)

// pipesout is a helper method to open stdout and stderr pipes and return them.
func pipesout(exe Executor) (io.Reader, io.Reader, error) {
	pstdout, err := exe.StdoutPipe()
	if err != nil {
		return nil, nil, exe.Errorf("error opening stdout pipe: %v", err)
	}
	pstderr, err := exe.StderrPipe()
	if err != nil {
		pstdout.Close()
		return nil, nil, exe.Errorf("error opening stderr pipe: %v", err)
	}
	return pstdout, pstderr, nil
}

// send creates and returns a source.  The exe contained therein will have already
// had Start() called on it.  Once a single value has been read from errchan it is
// safe to call exe.Wait, which is necessary to avoid resource leaks.
func send(exe Executor) (*source, error) {
	stdout, stderr, err := pipesout(exe)
	if err != nil {
		return nil, err
	}
	err = exe.Start()
	if err != nil {
		return nil, exe.Errorf("error starting pipe source: %v", err)
	}

	errchan := make(chan error)
	src := &source{exe: exe, stdout: stdout, errchan: errchan}
	go copyClose(&src.stderr, stderr, errchan)
	return src, nil
}

// recv creates and returns a sink.  The exe contained therein will have already
// had Start() called on it.  Once two values have been read from errchan it is
// safe to call exe.Wait, which is necessary to avoid resource leaks.
func recv(exe Executor) (*sink, error) {
	stdout, stderr, err := pipesout(exe)
	if err != nil {
		return nil, err
	}
	stdin, err := exe.StdinPipe()
	if err != nil {
		return nil, exe.Errorf("error creating stdin pipe: %v", err)
	}
	err = exe.Start()
	if err != nil {
		return nil, exe.Errorf("error starting pipe sink: %v", err)
	}

	errchan := make(chan error)
	snk := &sink{exe: exe, stdin: stdin, errchan: errchan}
	go copyClose(&snk.stderr, stderr, errchan)
	go copyClose(&snk.stdout, stdout, errchan)
	return snk, nil
}

// Pipe invokes two commands and connects the stdout of the source
// to the stdin of the sink.
func Pipe(srclch, snklch Launchable) PipeResult {
	srcexe, err := srclch.LaunchCmd()
	if err != nil {
		return PipeResult{Err: srclch.Errorf("error creating pipe source: %v", err)}
	}

	snkexe, err := snklch.LaunchCmd()
	if err != nil {
		return PipeResult{Err: snklch.Errorf("error creating pipe sink: %v", err)}
	}

	src, err := send(srcexe)
	if err != nil {
		return PipeResult{Err: err}
	}

	snk, err := recv(snkexe)
	if err != nil {
		// We won't bother reporting on errs produced during src shutdown, since
		// the sink never even started up successfully; that's the error we want
		// to report.  But should we report on a Kill() failure?
		_ = src.exe.Kill()
		// TODO once we support writing to stdin on the source, we must close stdin
		// before waiting.
		_ = src.exe.Wait()
		return PipeResult{Err: err}
	}

	return pipe{src, snk}.run()
}

// readandwrite does all the I/O but stops short of the Wait.
func (p pipe) readandwrite() error {
	errs := make(chan error)
	go func() {
		_, err := io.Copy(p.snk.stdin, p.src.stdout)
		errs <- err
	}()

	// Collect the results of the 4 I/Os: src stderr, snk stdout/stderr,
	// and the actual pipe copy from snk's stdin to src's stdout.
	// Return the first error found.  Close the pipe (i.e. sink's stdin)
	// once source drained so that sink doesn't hang around indefinitely.
	err, dones := error(nil), 0
	for dones < 4 {
		select {
		case err = <-errs:
			if err != nil {
				err = fmt.Errorf("error piping: %v", err)
			}
			dones++
			p.snk.stdin.Close()
		case srcerr := <-p.src.errchan:
			if srcerr != nil {
				err = fmt.Errorf("source error: %v", srcerr)
			}
			dones++
		case snkerr := <-p.snk.errchan:
			if snkerr != nil {
				err = fmt.Errorf("sink error: %v", snkerr)
			}
			dones++
		}
	}
	return err
}

// joinerrs returns nil if all errs are nil, otherwise a sep-separated
// concatenation of all non-nil errs.
func joinerrs(sep string, errs ...error) error {
	errstr := ""
	for _, e := range errs {
		if e != nil {
			errstr += fmt.Sprintf("%s%v", sep, e)
		}
	}
	if len(errstr) > 0 {
		return fmt.Errorf("%v", errstr[len(sep):])
	}
	return nil
}

func (p pipe) wait() error {
	// Order in which source/sink exit unspecified, so spawn goroutines
	// to collect the results and sync via channel.
	waitchan := make(chan error)
	go func() {
		err := p.src.exe.Wait()
		if err != nil {
			err = fmt.Errorf("source exited with error: %v", err)
		}
		waitchan <- err
	}()
	go func() {
		err := p.snk.exe.Wait()
		if err != nil {
			err = fmt.Errorf("sink exited with error: %v", err)
			p.src.exe.Kill()
		}
		waitchan <- err
	}()

	return joinerrs("; ", <-waitchan, <-waitchan)
}

func (p pipe) run() PipeResult {
	pr := PipeResult{Err: joinerrs("; ", p.readandwrite(), p.wait())}
	pr.SrcStderr = p.src.stderr.String()
	pr.SnkStderr = p.snk.stderr.String()
	pr.SnkStdout = p.snk.stdout.String()

	return pr
}
