package cmds

import (
	"context"
	"io"
	"io/ioutil"
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

// GetLocalEncoder provides special treatment for text encoding
// when Command.DisplayCLI field is non-nil, by defining an
// Encoder that delegates to a nested emitter that consumes a Response
// and writes to the underlying io.Writer using DisplayCLI.
func GetLocalEncoder(req *Request, w io.Writer, def EncodingType) (EncodingType, Encoder, error) {
	encType, enc, err := GetEncoder(req, w, def)
	if err != nil {
		return encType, nil, err
	}

	if req.Command.DisplayCLI != nil && encType == Text {
		emitter, response := NewChanResponsePair(req)
		go req.Command.DisplayCLI(response, w, ioutil.Discard)
		return encType, &emitterEncoder{emitter: emitter}, nil
	}

	return encType, enc, nil
}

type emitterEncoder struct {
	emitter ResponseEmitter
}

func (enc *emitterEncoder) Encode(value interface{}) error {
	return enc.emitter.Emit(value)
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
	maybeStartPostRun := func(formatters PostRunMap) <-chan error {
		var (
			postRun   func(Response, ResponseEmitter) error
			postRunCh = make(chan error)
		)

		if postRun == nil {
			close(postRunCh)
			return postRunCh
		}

		// check if we have a formatter for this emitter type
		typer, isTyper := re.(interface {
			Type() PostRunType
		})
		if isTyper &&
			formatters[typer.Type()] != nil {
			postRun = formatters[typer.Type()]
		} else {
			close(postRunCh)
			return postRunCh
		}

		// redirect emitter to us
		// and start waiting for emissions
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
	runCloseErr := re.CloseWithError(cmd.Run(req, re, env))
	postCloseErr := <-postRunCh
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
	return nil
}
