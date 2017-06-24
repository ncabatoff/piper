The main purpose of this package is to provide the Pipe function,
and to allow that to operate on different kinds of pipes - at
present, local pipes created with os/exec and ssh pipes created
with golang.org/x/crypto/ssh.

There are a few more basic non-pipe operations provided:

RunCmd executes a command with stdout going to /dev/null.  stderr
is captured and included in the error returned if the command exits
with non zero return status.

RunCmdStrIn is like RunCmd but accepts a string which will be
written to the stdin of the command.

RunCmdCapture is like RunCmd but also returns the stdout and
stderr.

RunCmdStrInCapture is like RunCmdStrIn but also returns the stdout
and stderr.
