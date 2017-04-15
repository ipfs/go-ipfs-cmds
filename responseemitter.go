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
	defer log.Debug("Copy returned.")

	log.Debugf("copy from %T to %T", res, re)
	re.SetLength(res.Length())

	for {
		if res.Error() != nil {
			e := res.Error()
			log.Debugf("Copy: copying error `%v` to a ResponseEmitter of type %T", e, re)
			re.SetError(e.Message, e.Code)
			return nil
		}

		v, err := res.Next()
		log.Debug("copy ", v, err)
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
