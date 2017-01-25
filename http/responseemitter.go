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
		err = re.preemble(value)

		if err == HeadRequest {
			re.headRequest = true
		} else if err != nil {
			return err
		}
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

func (re *ResponseEmitter) preemble(value interface{}) error {
	h := re.w.Header()
	// Expose our agent to allow identification
	h.Set("Server", "go-ipfs/"+config.CurrentVersionNumber)

	mime, err := guessMimeType(re.encType)
	if err != nil {
		http.Error(re.w, err.Error(), http.StatusInternalServerError)
		return err
	}

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

	fmt.Println("re.Length", re.Length())
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

func guessMimeType(enc cmds.EncodingType) (string, error) {
	// Try to guess mimeType from the encoding option
	if m, ok := mimeTypes[enc]; ok {
		return m, nil
	}

	return mimeTypes[cmds.JSON], nil
}
