package cmds

import (
	"io"
	"runtime/debug"

	"github.com/ipfs/go-ipfs-cmdkit"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	Request() *Request

	Error() *cmdkit.Error
	Length() uint64

	// Next returns the next emitted value.
	// The returned error can be a network or decoding error.
	// The error can also be ErrRcvdError if an error has been emitted.
	// In this case the emitted error can be accessed using the Error() method.
	Next() (interface{}, error)
}

type Head struct {
	Len uint64
	Err *cmdkit.Error
}

func (h Head) Length() uint64 {
	return h.Len
}

func (h Head) Error() *cmdkit.Error {
	return h.Err
}

// HandleError handles the error from cmds.Response.Next(), it returns
// true if Next() should be called again
func HandleError(err error, res Response, re ResponseEmitter) bool {
	if err != nil {
		if err == io.EOF {
			return false
		}

		closeErr := re.CloseWithError(err)
		if err != nil {
			log.Errorf("error closing with error: %v at %s when closing with error %v", closeErr, debug.Stack(), err)
		}
		return false
	}
	return true
}
