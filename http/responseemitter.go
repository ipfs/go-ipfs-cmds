package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	cmds "github.com/ipfs/go-ipfs/commands"
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
	w           http.ResponseWriter
	enc         cmds.Encoder
	encType     cmds.EncodingType
	method      string
	err         error
	hasEmitted  bool
	headRequest bool
	length      uint64
}

func (re *ResponseEmitter) Emit(value interface{}) error {
	var err error

	if !re.hasEmitted {
		re.hasEmitted = true
		err = re.preamble(value)

		if err == HeadRequest {
			re.headRequest = true
		} else if err != nil {
			return err
		}
	}

	// ignore those
	if value == nil {
		return nil
	}

	// return immediately if this is a head request
	if re.headRequest {
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

func (re *ResponseEmitter) Length() uint64 {
	return re.length
}

func (re *ResponseEmitter) Close() error {
	// can't close HTTP connections
	return nil
}

func (re *ResponseEmitter) SetError(err interface{}, code cmds.ErrorType) {
	var str string

	if err_, ok := err.(error); ok {
		str = err_.Error()
	} else {
		str = fmt.Sprintf("%v", err)
	}

	re.err = &cmds.Error{Message: str, Code: code}
	re.Emit(re.err)
}

func (re *ResponseEmitter) Stdout() io.Writer {
	return os.Stdout
}

func (re *ResponseEmitter) Stderr() io.Writer {
	return os.Stderr
}

func (re *ResponseEmitter) Flush() {
	re.w.(http.Flusher).Flush()
}

func (re *ResponseEmitter) preamble(value interface{}) error {
	h := re.w.Header()
	// Expose our agent to allow identification
	h.Set("Server", "go-ipfs/"+config.CurrentVersionNumber)

	mime := guessMimeType(re.encType)

	status := http.StatusOK
	// if response contains an error, write an HTTP error status code
	if e := re.err; e != nil {
		if e.(cmds.Error).Code == cmds.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
		// NOTE: The error will actually be written out by the reader below
	}

	//	out, err := res.Reader()
	//	if err != nil {
	//		http.Error(w, err.Error(), http.StatusInternalServerError)
	//		return
	//	}

	// Set up our potential trailer
	h.Set("Trailer", StreamErrHeader)

	if re.Length() > 0 {
		h.Set("X-Content-Length", strconv.FormatUint(re.Length(), 10))
	}

	// TODO see if this is really the right thing to check. Maybe re.Length() == 0?
	if _, ok := value.(io.Reader); ok {
		// set streams output type to text to avoid issues with browsers rendering
		// html pages on priveleged api ports
		mime = "text/plain"
		h.Set(streamHeader, "1")
	}

	/* TODO can't check for that...generally find out how to check which encoding to use. We don't really have channel vs reader here.
	// if output is a channel and user requested streaming channels,
	// use chunk copier for the output
	_, isChan := res.Output().(chan interface{})
	if !isChan {
		_, isChan = res.Output().(<-chan interface{})
	}

	if isChan {
		h.Set(channelHeader, "1")
	}
	*/

	// catch-all, set to text as default
	if mime == "" {
		mime = "text/plain"
	}

	h.Set(contentTypeHeader, mime)

	// set 'allowed' headers
	h.Set("Access-Control-Allow-Headers", AllowedExposedHeaders)
	// expose those headers
	h.Set("Access-Control-Expose-Headers", AllowedExposedHeaders)

	if re.method == "HEAD" { // after all the headers.
		return HeadRequest
	}

	re.w.WriteHeader(status)

	return nil
}

func guessMimeType(enc cmds.EncodingType) string {
	// Try to guess mimeType from the encoding option
	if m, ok := mimeTypes[enc]; ok {
		return m
	}

	return mimeTypes[cmds.JSON]
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
