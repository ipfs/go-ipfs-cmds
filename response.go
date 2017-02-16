package cmds

import (
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	Request() Request

	Error() *cmdsutil.Error
	Length() uint64

	Next() (interface{}, error)
}

type Head struct {
	Len uint64
	Err *cmdsutil.Error
}

func (h Head) Length() uint64 {
	return h.Len
}

func (h Head) Error() *cmdsutil.Error {
	return h.Err
}
