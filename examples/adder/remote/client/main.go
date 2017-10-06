package main

import (
	"os"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmezbW7VUAiu3aSV6r4TdB9pwficnnbtWYKRsoEKF2w8G2/go-ipfs-cmds"

	cli "gx/ipfs/QmezbW7VUAiu3aSV6r4TdB9pwficnnbtWYKRsoEKF2w8G2/go-ipfs-cmds/cli"
	http "gx/ipfs/QmezbW7VUAiu3aSV6r4TdB9pwficnnbtWYKRsoEKF2w8G2/go-ipfs-cmds/http"
)

func main() {
	// parse the command path, arguments and options from the command line
	req, cmd, _, err := cli.Parse(os.Args[1:], os.Stdin, adder.RootCmd)
	if err != nil {
		panic(err)
	}

	// create http rpc client
	client := http.NewClient(":6798")

	// send request to server
	res, err := client.Send(req)
	if err != nil {
		panic(err)
	}

	// create an emitter
	re, retCh := cli.NewResponseEmitter(os.Stdout, os.Stderr, cmd.Encoders["Text"], req)

	wait := make(chan struct{})
	// copy received result into cli emitter
	go func() {
		err = cmds.Copy(re, res)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal|cmdkit.ErrFatal)
		}
		close(wait)
	}()

	// wait until command has returned and exit
	ret := <-retCh
	<-wait
	os.Exit(ret)
}
