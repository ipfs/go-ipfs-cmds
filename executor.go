package cmds

import (
	"context"
	"runtime/debug"
)

type Executor interface {
	Execute(req *Request, re ResponseEmitter, env Environment) error
}

// Environment is the environment passed to commands. The only required method
// is Context.
type Environment interface {
	// Context returns the environment's context.
	Context() context.Context
}

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

func (x *executor) Execute(req *Request, re ResponseEmitter, env Environment) (err error) {
	cmd := req.Command

	if cmd.Run == nil {
		return ErrNotCallable
	}

	err = cmd.CheckArguments(req)
	if err != nil {
		return err
	}

	// If this ResponseEmitter encodes messages (e.g. http, cli or writer - but not chan),
	// we need to update the encoding to the one specified by the command.
	if ee, ok := re.(EncodingEmitter); ok {
		encType := GetEncoding(req)

		// use JSON if text was requested but the command doesn't have a text-encoder
		if _, ok := cmd.Encoders[encType]; encType == Text && !ok {
			encType = JSON
		}

		if enc, ok := cmd.Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else if enc, ok := Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else {
			log.Errorf("unknown encoding %q, using json", encType)
			ee.SetEncoder(Encoders[JSON](req))
		}
	}

	if cmd.PreRun != nil {
		err = cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	if cmd.PostRun != nil {
		if typer, ok := re.(interface {
			Type() PostRunType
		}); ok && cmd.PostRun[typer.Type()] != nil {
			var (
				res   Response
				lower = re
			)

			re, res = NewChanResponsePair(req)

			go func() {
				var closeErr error
				err := cmd.PostRun[typer.Type()](res, lower)
				if err != nil {
					closeErr = lower.CloseWithError(err)
				} else {
					closeErr = lower.Close()
				}

				if closeErr != nil {
					log.Errorf("error closing connection: %s", closeErr)
					if err != nil {
						log.Errorf("close caused by error: %s", err)
					}
				}
			}()
		}
	}

	defer func() {
		// catch panics in Run (esp. from re.SetError)
		if v := recover(); v != nil {
			log.Errorf("panic in command handler at %s", debug.Stack())

			// if they are errors
			if err, ok := v.(error); ok {
				// use them as return error
				closeErr := re.CloseWithError(err)
				if closeErr != nil {
					log.Errorf("error closing connection: %s", closeErr)
					if err != nil {
						log.Errorf("close caused by error: %s", err)
					}
				}
			} else {
				// otherwise keep panicking.
				panic(v)
			}
		}

	}()
	err = cmd.Run(req, re, env)
	log.Debugf("cmds.Execute: Run returned %q with response emitter of type %T", err, re)
	if err == nil {
		return re.Close()
	}


	return re.CloseWithError(err)
}
