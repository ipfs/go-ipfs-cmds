package cmds

import (
	"io"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
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
	SetError(err interface{}, code cmdsutil.ErrorType)

	// Gets Stdout and Stderr, for writing to console without using SetOutput
	// TODO I'm not sure we really need that, but lets see
	//Stdout() io.Writer
	//Stderr() io.Writer

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
		if res.Error() != nil {
			e := res.Error()
			log.Debugf("Copy: copying error `%v` to a ResponseEmitter of type %T", e, re)
			re.SetError(e.Message, e.Code)
			return nil
		} else {
			log.Debugf("Copy: Response of type %T has no error. ResponseEmitter is of type %T", res, re)
		}

		v, err := res.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		err = re.Emit(v)
		if err != nil {
			return err
		}
	}
}
