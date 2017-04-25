package cmds

import (
	"io"

	"gx/ipfs/QmadYQbq2fJpaRE3XhpMLH68NNxmWMwfMQy1ntr1cKf7eo/go-ipfs-cmdkit"
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
	SetError(err interface{}, code cmdsutil.ErrorType) error

	// Gets Stdout and Stderr, for writing to console without using SetOutput
	// TODO I'm not sure we really need that, but lets see
	//Stdout() io.Writer
	//Stderr() io.Writer

	// Tee makes this Responseemitter forward all calls to SetError, SetLength and
	// Emit to the passed ResponseEmitter
	//Tee(ResponseEmitter)

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
		if err == io.EOF {
			re.Close()
			return nil
		}
		if err == ErrRcvdError {
			re.SetError(res.Error().Message, res.Error().Code)
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
