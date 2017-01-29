package cmds

import (
	"io"
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
	SetError(err interface{}, code ErrorType)

	// Gets Stdout and Stderr, for writing to console without using SetOutput
	// TODO I'm not sure we really need that, but lets see
	//Stdout() io.Writer
	//Stderr() io.Writer

	// Emit sends a value
	// if value is io.Reader we just copy that to the connection
	// other values are marshalled
	Emit(value interface{}) error
}
