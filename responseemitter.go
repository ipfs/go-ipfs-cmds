package commands

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Encoder encodes values onto e.g. an io.Writer. Examples are json.Encoder and xml.Encoder.
type Encoder interface {
	Encode(value interface{}) error
}

var encoders = map[EncodingType]func(w io.Writer) Encoder{
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
func NewResponseEmitter(w io.WriteCloser, encType EncodingType) ResponseEmitter {
	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     encoders[encType](w),
	}

	if _, ok := w.(http.ResponseWriter); ok {
		return &httpResponseEmitter{re}
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

// httpResponseEmitter is a ResponseEmitter specific to HTTP connections. Exposes flushing.
type httpResponseEmitter struct {
	ResponseEmitter
}

func (re *httpResponseEmitter) Flush() {
	// TODO review question: this is guaranteed to work but we'll panic if it doesn't. should I wrap that?
	re.ResponseEmitter.(*responseEmitter).w.(http.Flusher).Flush()
}
