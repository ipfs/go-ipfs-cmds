package main

import (
	"os"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	"gx/ipfs/QmezbW7VUAiu3aSV6r4TdB9pwficnnbtWYKRsoEKF2w8G2/go-ipfs-cmds/cli"
)

func main() {
	// parse the command path, arguments and options from the command line
	req, cmd, _, err := cli.Parse(os.Args[1:], os.Stdin, adder.RootCmd)
	if err != nil {
		panic(err)
	}

	// create an emitter
	re, retCh := cli.NewResponseEmitter(os.Stdout, os.Stderr, cmd.Encoders["Text"], req)

	// call command in background
	go func() {
		err = adder.RootCmd.Call(req, re)
		if err != nil {
			panic(err)
		}
	}()

	// wait until command has returned and exit
	os.Exit(<-retCh)
}
