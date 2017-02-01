package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"

	"github.com/ipfs/go-ipfs/repo/config"
)

var (
	HeadRequest = fmt.Errorf("HEAD request")
)

// NewResponeEmitter returns a new ResponseEmitter.
func NewResponseEmitter(w http.ResponseWriter, encType cmds.EncodingType, method string) *ResponseEmitter {
	re := &ResponseEmitter{
		w:       w,
		encType: encType,
		enc:     cmds.Encoders[encType](w),
		method:  method,
	}
	return re
}

type ResponseEmitter struct {
	w http.ResponseWriter

	enc     cmds.Encoder
	encType cmds.EncodingType

	length uint64
	err    *cmdsutil.Error

	hasEmitted bool
	method     string
}

func (re *ResponseEmitter) Emit(value interface{}) error {
	var err error

	if !re.hasEmitted {
		re.hasEmitted = true
		re.preamble(value)
	}

	// ignore those
	if value == nil {
		return nil
	}

	// return immediately if this is a head request
	if re.method == "HEAD" {
		return nil
	}

	// Special case: if text encoding and an error, just print it out.
	// TODO review question: its like that in response.go, should we keep that?
	if re.encType == cmds.Text && re.err != nil {
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

func (re *ResponseEmitter) SetLength(l uint64) {
	re.length = l
}

func (re *ResponseEmitter) Close() error {
	// can't close HTTP connections
	return nil
}

func (re *ResponseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	re.err = &cmdsutil.Error{Message: fmt.Sprint(err), Code: code}

	// force send of preamble
	// TODO is this the right thing to do?
	re.Emit(nil)
}

// Flush the http connection
func (re *ResponseEmitter) Flush() {
	if !re.hasEmitted {
		re.hasEmitted = true
		re.preamble(value)
	}

	re.w.(http.Flusher).Flush()
}

func (re *ResponseEmitter) preamble(value interface{}) {
	h := re.w.Header()
	// Expose our agent to allow identification
	h.Set("Server", "go-ipfs/"+config.CurrentVersionNumber)

	status := http.StatusOK
	// if response contains an error, write an HTTP error status code
	if e := re.err; e != nil {
		if e.Code == cmdsutil.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
		// NOTE: The error will actually be written out below
	}

	// write error to connection
	if re.err != nil {
		if re.err.Code == cmdsutil.ErrClient {
			http.Error(re.w, re.err.Error(), http.StatusInternalServerError)
		}

		return
	}

	// Set up our potential trailer
	h.Set("Trailer", StreamErrHeader)

	if re.Length() > 0 {
		h.Set("X-Content-Length", strconv.FormatUint(re.Length(), 10))
	}

	if _, ok := value.(io.Reader); ok {
		// set streams output type to text to avoid issues with browsers rendering
		// html pages on priveleged api ports
		h.Set(streamHeader, "1")
	} else {
		h.Set(channelHeader, "1")
	}

	// lookup mime type from map
	mime := mimeTypes[re.encType]

	// catch-all, set to text as default
	if mime == "" {
		mime = "text/plain"
	}

	h.Set(contentTypeHeader, mime)

	// set 'allowed' headers
	h.Set("Access-Control-Allow-Headers", AllowedExposedHeaders)
	// expose those headers
	h.Set("Access-Control-Expose-Headers", AllowedExposedHeaders)

	re.w.WriteHeader(status)
}

// NewTeeEmitter returns a ResponseEmitter that can Flush. It will forward that flush to re1 and, if it can flush, re2.
func NewTeeEmitter(re1 *ResponseEmitter, re2 cmds.ResponseEmitter) *ResponseEmitter {
	return *teeEmitter{
		re1, re2,
	}
}

type teeEmitter struct {
	*ResponseEmitter

	re cmds.ResponseEmitter
}

func (re *teeEmitter) Close() error {
	err1 := re.ResponseEmitter.Close()
	err2 := re.re.Close()

	if err1 != nil {
		return err1
	}

	// XXX we drop the second error if both fail
	return err2
}

func (re *teeEmitter) Emit(v interface{}) error {
	err1 := re.ResponseEmitter.Emit()
	err2 := re.re.Emit()

	if err1 != nil {
		return err1
	}

	// XXX we drop the second error if both fail
	return err2
}

func (re *teeEmitter) SetError(err interface{}, code cmds.ErrorType) {
	re.ResponseEmitter.SetError(err, code)
	re.re.SetError(err, code)
}

func (re *teeEmitter) Flush() {
	re.ResponseEmitter.Flush()

	if hre, ok := re.re.(*ResponseEmitter); ok {
		hre.Flush()
	}
}
