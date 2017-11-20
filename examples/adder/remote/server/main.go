package main

import (
	nethttp "net/http"

	"github.com/ipfs/go-ipfs/core"

	"github.com/ipfs/go-ipfs-cmds/examples/adder"

	cmds "github.com/ipfs/go-ipfs-cmds"
	http "github.com/ipfs/go-ipfs-cmds/http"
)

func main() {
	h := http.NewHandler(
		cmds.Context{
			ConstructNode: func() (*core.IpfsNode, error) {
				return &core.IpfsNode{}, nil
			},
		},
		adder.RootCmd,
		http.NewServerConfig())

	// create http rpc server
	err := nethttp.ListenAndServe(":6798", h)
	if err != nil {
		panic(err)
	}
}
