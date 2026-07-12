package main

import (
	"context"
	"os"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
)

func main() {
	// parse the command path, arguments and options from the command line
	req, err := cli.Parse(context.TODO(), os.Args[1:], os.Stdin, adder.RootCmd)
	if err != nil {
		panic(err)
	}

	req.Options["encoding"] = cmds.Text

	// create an emitter
	cliRe, err := cli.NewResponseEmitter(os.Stdout, os.Stderr, req)
	if err != nil {
		panic(err)
	}

	exec := cmds.NewExecutor(adder.RootCmd)
	err = exec.Execute(req, cliRe, nil)
	if err != nil {
		panic(err)
	}

	os.Exit(cliRe.Status())
}
