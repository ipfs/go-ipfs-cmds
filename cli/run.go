package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmYopJAcV7R9SbxiPBCvqhnt8EusQpWPHewoZakCMt8hps/go-ipfs-cmds"
)

type Closer interface {
	Close()
}

func Run(ctx context.Context, root *cmds.Command,
	cmdline []string, stdin, stdout, stderr *os.File,
	buildEnv func(context.Context, *cmds.Request) interface{},
	makeExecutor func(*cmds.Request, interface{}) (cmds.Executor, error)) error {

	printErr := func(err error) {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
	}

	req, errParse := Parse(ctx, cmdline[1:], stdin, root)

	// this is a message to tell the user how to get the help text
	printMetaHelp := func(w io.Writer) {
		cmdPath := strings.Join(req.Path, " ")
		fmt.Fprintf(w, "Use '%s %s --help' for information about this command\n", cmdline[0], cmdPath)
	}

	printHelp := func(long bool, w io.Writer) {
		helpFunc := ShortHelp
		if long {
			helpFunc = LongHelp
		}

		var path []string
		if req != nil {
			path = req.Path
		}

		helpFunc(cmdline[0], root, path, w)
	}

	// BEFORE handling the parse error, if we have enough information
	// AND the user requested help, print it out and exit
	err := HandleHelp(cmdline[0], req, stdout)
	if err == nil {
		return nil
	} else if err != ErrNoHelpRequested {
		return err
	}
	// no help requested, continue.

	// ok now handle parse error (which means cli input was wrong,
	// e.g. incorrect number of args, or nonexistent subcommand)
	if errParse != nil {
		printErr(errParse)

		// this was a user error, print help
		if req != nil && req.Command != nil {
			fmt.Fprintln(stderr) // i need some space
			printHelp(false, stderr)
		}

		return err
	}

	// here we handle the cases where
	// - commands with no Run func are invoked directly.
	// - the main command is invoked.
	if req == nil || req.Command == nil || req.Command.Run == nil {
		printHelp(false, stdout)
		return nil
	}

	cmd := req.Command

	env := buildEnv(ctx, req)
	if c, ok := env.(Closer); ok {
		defer c.Close()
	}

	exctr, err := makeExecutor(req, env)
	if err != nil {
		printErr(err)
		return err
	}

	var (
		re     cmds.ResponseEmitter
		exitCh <-chan int
	)

	encTypeStr, _ := req.Options[cmds.EncLong].(string)
	encType := cmds.EncodingType(encTypeStr)

	// first if condition checks the command's encoder map, second checks global encoder map (cmd vs. cmds)
	if enc, ok := cmd.Encoders[encType]; ok {
		re, exitCh = NewResponseEmitter(stdout, stderr, enc, req)
	} else if enc, ok := cmds.Encoders[encType]; ok {
		re, exitCh = NewResponseEmitter(stdout, stderr, enc, req)
	} else {
		return fmt.Errorf("could not find matching encoder for enctype %#v", encType)
	}

	err = exctr.Execute(req, re, env)
	if err != nil {
		if kiterr, ok := err.(*cmdkit.Error); ok {
			err = *kiterr
		}
		if kiterr, ok := err.(cmdkit.Error); ok && kiterr.Code == cmdkit.ErrClient {
			printMetaHelp(stderr)
		}

		return err
	}

	if code := <-exitCh; code != 0 {
		err = exitErr(code)
	}

	return err
}

type exitErr int

func (e exitErr) Error() string {
	return fmt.Sprint("exit code", int(e))
}
