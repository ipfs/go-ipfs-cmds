package cmds

import (
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	// TODO should be drop that?
	Request() *Request

	Error() *cmdsutil.Error
	Length() uint64

	Next() (interface{}, error)
}
