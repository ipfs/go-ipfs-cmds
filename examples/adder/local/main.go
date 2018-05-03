package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	"github.com/ipfs/go-ipfs-cmds"
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
	re, retCh := cli.NewResponseEmitter(os.Stdout, os.Stderr, req.Command.Encoders["Text"], req)

	if pr, ok := req.Command.PostRun[cmds.CLI]; ok {
		var (
			res   cmds.Response
			lower = re
		)

		re, res = cmds.NewChanResponsePair(req)

		go func() {
			err := pr(res, lower)
			if err != nil {
				fmt.Println("error: ", err)
			}
		}()
	}

	wait := make(chan struct{})
	// call command in background
	go func() {
		defer close(wait)

		adder.RootCmd.Call(req, re, nil)
	}()

	// wait until command has returned and exit
	ret := <-retCh
	<-wait

	os.Exit(ret)
}
