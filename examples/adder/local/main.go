package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
)

/* TODO:
- there's too many words in here
- text style consistency; sleepy case is currently used with Chicago norm expected as majority
- We should also move a lot of this into the subcommand definition and refer to that
instead, of documenting internals in very big comments
- this would allow us to utilize env inside of `Run` which is important for demonstration
- Need to trace this when done and make sure the comments are telling the truth about the execution path
- Should probably throw in a custom emitter example somewhere since we defined one elsewhere
- and many more
*/

// `cli.Run` will read a `cmds.Command`'s fields and perform standard behaviour
// for package defined values.
// For example, if the `Command` declares it has `cmds.OptLongHelp` or `cmds.OptShortHelp` options,
// `cli.Run` will check for and handle these flags automatically (but they are not required).
//
// If needed, the caller may define an arbitrary "environment"
// and expect to receive this environment during execution of the `Command`.
// While not required we'll define one for the sake of example.
type (
	envChan chan error

	ourEnviornment struct {
		whateverWeNeed envChan
	}
)

// A `Close` method is also optional; if defined, it's deferred inside of `cli.Run`.
func (env *ourEnviornment) Close() error {
	if env.whateverWeNeed != nil {
		close(env.whateverWeNeed)
	}
	return nil
}

// If desired, the caller may define additional methods that
// they may need during execution of the `cmds.Command`.
// Considering the environment constructor returns an untyped interface,
// it's a good idea to define additional interfaces that can be used for behaviour checking.
// (Especially if you choose to return different concrete environments for different requests.)
func (env *ourEnviornment) getChan() envChan {
	return env.whateverWeNeed
}

type specificEnvironment interface {
	getChan() envChan
}

// While the environment itself is not be required,
// its constructor and receiver methods are.
// We'll define them here without any special request parsing, since we don't need it.
func makeOurEnvironment(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
	return &ourEnviornment{
		whateverWeNeed: make(envChan),
	}, nil
}
func makeOurExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	return cmds.NewExecutor(adder.RootCmd), nil
}

func main() {
	var (
		ctx = context.TODO()
		// If the environment constructor does not return an error
		// it will pass the environment to the `cmds.Executor` within `cli.Run`;
		// which passes it to the `Command`'s (optional)`PreRun` and/or `Run` methods.
		err = cli.Run(ctx, adder.RootCmd, os.Args, // pass in command and args to parse
			os.Stdin, os.Stdout, os.Stderr, // along with output writers
			makeOurEnvironment, makeOurExecutor) // and our constructor+receiver pair
	)
	cliError := new(cli.ExitError)
	if errors.As(err, cliError) {
		os.Exit(int((*cliError)))
	}
}

// `cli.Run` is a convenient wrapper and not required.
// If desired, the caller may define the entire means to process a `cmds.Command` themselves.
func altMain() {
	ctx := context.TODO()

	// parse the command path, arguments and options from the command line
	request, err := cli.Parse(ctx, os.Args[1:], os.Stdin, adder.RootCmd)
	if err != nil {
		panic(err)
	}
	request.Options[cmds.EncLong] = cmds.Text

	// create an environment from the request
	cmdEnv, err := makeOurEnvironment(ctx, request)
	if err != nil {
		panic(err)
	}

	// get values specific to our request+environment pair
	wait, err := customPreRun(request, cmdEnv)
	if err != nil {
		panic(err)
	}

	// This emitter's `Emit` method will be called from within the `Command`'s `Run` method.
	// If `Run` encounters a fatal error, the emitter should be closed with `emitter.CloseWithError(err)`
	// otherwise, it will be closed automatically after `Run` within `Call`.
	var emitter cmds.ResponseEmitter
	emitter, err = cli.NewResponseEmitter(os.Stdout, os.Stderr, request)
	if err != nil {
		panic(err)
	}

	// if the command has a `PostRun` method, emit responses to it instead
	emitter = maybePostRun(request, emitter, wait)

	// call the actual `Run` method on the command
	adder.RootCmd.Call(request, emitter, cmdEnv)
	err = <-wait

	cliError := new(cli.ExitError)
	if errors.As(err, cliError) {
		os.Exit(int((*cliError)))
	}
}

func customPreRun(req *cmds.Request, env cmds.Environment) (envChan, error) {
	// check that the constructor passed us the environment we expect/need
	ourEnvIntf, ok := env.(specificEnvironment)
	if !ok {
		return nil, fmt.Errorf("environment received does not satisfy expected interface")
	}
	return ourEnvIntf.getChan(), nil
}

func maybePostRun(req *cmds.Request, emitter cmds.ResponseEmitter, wait envChan) cmds.ResponseEmitter {
	postRun, provided := req.Command.PostRun[cmds.CLI]
	if !provided { // no `PostRun` command was defined
		close(wait) // don't do anything and unblock instantly
		return emitter
	}

	var ( // store the emitter passed to us
		postRunEmitter = emitter
		response       cmds.Response
	)
	// replace the caller's emitter with one that emits to this `Response` interface
	emitter, response = cmds.NewChanResponsePair(req)

	go func() { // start listening for emission on the emitter
		// wait for `PostRun` to return, and send its value to the caller
		wait <- postRun(response, postRunEmitter)
		close(wait)
	}()

	return emitter
}
