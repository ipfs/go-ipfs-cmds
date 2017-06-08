package cmds

import (
	"io"

	"github.com/ipfs/go-ipfs-cmdkit"
)

// ResponseEmitter encodes and sends the command code's output to the client.
// It is all a command can write to.
type ResponseEmitter interface {
	// closes http conn or channel
	io.Closer

	// SetLength sets the length of the output
	// err is an interface{} so we don't have to manually convert to error.
	SetLength(length uint64)

	// SetError sets the response error
	// err is an interface{} so we don't have to manually convert to error.
	SetError(err interface{}, code cmdkit.ErrorType)

	// Emit sends a value
	// if value is io.Reader we just copy that to the connection
	// other values are marshalled
	Emit(value interface{}) error
}

type EncodingEmitter interface {
	ResponseEmitter

	SetEncoder(func(io.Writer) Encoder)
}

type Header interface {
	Head() Head
}

func Copy(re ResponseEmitter, res Response) error {
	re.SetLength(res.Length())

	for {
		v, err := res.Next()
		switch err {
		case nil:
			// all good, go on
		case io.EOF:
			re.Close()
			return nil
		case ErrRcvdError:
			re.Emit(res.Error())
			continue
		default:
			return err
		}

		err = re.Emit(v)
		if err != nil {
			return err
		}
	}
}
