package cmds

import (
	"context"
	"os"
)

type Executor interface {
	Execute(req *Request, re ResponseEmitter, env Environment) error
}

// Environment is the environment passed to commands.
type Environment interface{}

// MakeEnvironment takes a context and the request to construct the environment
// that is passed to the command's Run function.
// The user can define a function like this to pass it to cli.Run.
type MakeEnvironment func(context.Context, *Request) (Environment, error)

// MakeExecutor takes the request and environment variable to construct the
// executor that determines how to call the command - i.e. by calling Run or
// making an API request to a daemon.
// The user can define a function like this to pass it to cli.Run.
type MakeExecutor func(*Request, interface{}) (Executor, error)

func NewExecutor(root *Command) Executor {
	return &executor{
		root: root,
	}
}

type executor struct {
	root *Command
}

func (x *executor) Execute(req *Request, re ResponseEmitter, env Environment) error {
	cmd := req.Command

	if cmd.Run == nil {
		return ErrNotCallable
	}

	err := cmd.CheckArguments(req)
	if err != nil {
		return err
	}

	if cmd.PreRun != nil {
		err = cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	return EmitResponse(cmd.Run, req, re, env)
}

// Helper for Execute that handles post-Run emitter logic
func EmitResponse(run Function, req *Request, re ResponseEmitter, env Environment) error {

	// Keep track of the lowest emitter to select the correct
	// PostRun method.
	lowest := re
	cmd := req.Command

	// contains the error returned by DisplayCLI or PostRun
	errCh := make(chan error, 1)

	if cmd.DisplayCLI != nil && GetEncoding(req, "json") == "text" {
		var res Response

		// This overwrites the emitter provided as an
		// argument. Maybe it's better to provide the
		// 'DisplayCLI emitter' as an argument to Execute.
		re, res = NewChanResponsePair(req)

		go func() {
			defer close(errCh)
			errCh <- cmd.DisplayCLI(res, os.Stdout, os.Stderr)
		}()
	} else {
		close(errCh)
	}

	maybeStartPostRun := func(formatters PostRunMap) <-chan error {
		var (
			postRun   func(Response, ResponseEmitter) error
			postRunCh = make(chan error)
		)

		// Check if we have a formatter for this emitter type.
		typer, isTyper := lowest.(interface {
			Type() PostRunType
		})
		if isTyper {
			postRun = formatters[typer.Type()]
		}
		// If not, just return nil via closing.
		if postRun == nil {
			close(postRunCh)
			return postRunCh
		}

		// Otherwise, relay emitter responses
		// from Run to PostRun, and
		// from PostRun to the original emitter.
		var (
			postRes     Response
			postEmitter = re
		)
		re, postRes = NewChanResponsePair(req)
		go func() {
			defer close(postRunCh)
			postRunCh <- postEmitter.CloseWithError(postRun(postRes, postEmitter))
		}()
		return postRunCh
	}

	postRunCh := maybeStartPostRun(cmd.PostRun)
	runCloseErr := re.CloseWithError(run(req, re, env))
	postCloseErr := <-postRunCh
	displayCloseErr := <-errCh
	
	switch runCloseErr {
	case ErrClosingClosedEmitter, nil:
	default:
		return runCloseErr
	}
	switch postCloseErr {
	case ErrClosingClosedEmitter, nil:
	default:
		return postCloseErr
	}
	return displayCloseErr
}
