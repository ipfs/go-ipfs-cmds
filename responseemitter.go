package commands

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// Encoder encodes values onto e.g. an io.Writer. Examples are json.Encoder and xml.Encoder.
type Encoder interface {
	Encode(value interface{}) error
}

var Encoders = map[EncodingType]func(w io.Writer) Encoder{
	XML: func(w io.Writer) Encoder {
		return xml.NewEncoder(w)
	},
	JSON: func(w io.Writer) Encoder {
		return json.NewEncoder(w)
	},
}

// ResponseEmitter encodes and sends the command code's output to the client.
// It is all a command can write to.
type ResponseEmitter interface {
	// closes http conn or channel
	io.Closer

	// Set/Return the response Error
	// err is an interface{} so we don't have to manually convert to error.
	SetError(err interface{}, code ErrorType)

	// Gets Stdout and Stderr, for writing to console without using SetOutput
	Stdout() io.Writer
	Stderr() io.Writer

	// send value
	// if value is io.Reader we just copy that to the connection
	// other values are marshalled
	Emit(value interface{}) error
}

// NewResponeEmitter returns a new ResponseEmitter.
// You should use "commands/http".ResponseEmitter for HTTP connections
func NewResponseEmitter(w io.WriteCloser, encType EncodingType) ResponseEmitter {
	//XXX possible change in behaviour, just temp
	enc, ok := Encoders[encType]
	if !ok {
		enc = Encoders[JSON]
	}

	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     enc(w),
	}

	return re
}

type responseEmitter struct {
	w       io.WriteCloser
	enc     Encoder
	encType EncodingType
	err     error
}

func (re *responseEmitter) Close() error {
	return re.w.Close()
}

func (re *responseEmitter) SetError(err interface{}, code ErrorType) {
	var str string

	if err_, ok := err.(error); ok {
		str = err_.Error()
	} else {
		str = fmt.Sprintf("%v", err)
	}

	re.err = &Error{Message: str, Code: code}
	re.Emit(re.err)
}

func (re *responseEmitter) Stdout() io.Writer {
	return os.Stdout
}

func (re *responseEmitter) Stderr() io.Writer {
	return os.Stderr
}

func (re *responseEmitter) Emit(value interface{}) error {
	var err error

	// Special case: if text encoding and an error, just print it out.
	// TODO review question: its like that in response.go, should we keep that?
	if re.encType == Text && re.err != nil {
		value = re.err
	}

	switch v := value.(type) {
	case io.Reader:
		_, err = io.Copy(re.w, v)
	default:
		err = re.enc.Encode(value)
	}

	return err
}
