package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"

	"github.com/ipfs/go-ipfs/repo/config"
)

var (
	HeadRequest = fmt.Errorf("HEAD request")
)

// NewResponeEmitter returns a new ResponseEmitter.
func NewResponseEmitter(w http.ResponseWriter, encType cmds.EncodingType, method string) ResponseEmitter {
	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     cmds.Encoders[encType](w),
		method:  method,
	}
	return re
}

type ResponseEmitter interface {
	cmds.ResponseEmitter
	http.Flusher
}

type responseEmitter struct {
	w http.ResponseWriter

	enc     cmds.Encoder
	encType cmds.EncodingType

	length uint64
	err    *cmdsutil.Error

	hasEmitted bool
	method     string
}

func (re *responseEmitter) Emit(value interface{}) error {
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

func (re *responseEmitter) SetLength(l uint64) {
	re.length = l
}

func (re *responseEmitter) Close() error {
	// can't close HTTP connections
	return nil
}

func (re *responseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	re.err = &cmdsutil.Error{Message: fmt.Sprint(err), Code: code}

	// force send of preamble
	// TODO is this the right thing to do?
	re.Emit(nil)
}

// Flush the http connection
func (re *responseEmitter) Flush() {
	if !re.hasEmitted {
		re.hasEmitted = true

		// setting this to nil means that it sends channel/chunked-encoding headers
		re.preamble(nil)
	}

	re.w.(http.Flusher).Flush()
}

func (re *responseEmitter) preamble(value interface{}) {
	log.Debug("re.preamble")

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

	if re.length > 0 {
		h.Set("X-Content-Length", strconv.FormatUint(re.length, 10))
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
