package http

import (
	"github.com/ipfs/go-ipfs-cmds"
	"net/http"
)

type flushfwder struct {
	cmds.ResponseEmitter
	http.Flusher
}

type FlushForwarder interface {
	cmds.ResponseEmitter
	http.Flusher
}

func NewFlushForwarder(r ResponseEmitter, f http.Flusher) HTTPResponseEmitter {
	return flushfwder{ResponseEmitter: r, Flusher: f}
}
