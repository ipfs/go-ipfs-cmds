package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/debug"
)

var (
	HeadRequest = fmt.Errorf("HEAD request")

	AllowedExposedHeadersArr = []string{streamHeader, channelHeader, extraContentLengthHeader}
	AllowedExposedHeaders    = strings.Join(AllowedExposedHeadersArr, ", ")

	mimeTypes = map[cmds.EncodingType]string{
		cmds.Protobuf: "application/protobuf",
		cmds.JSON:     "application/json",
		cmds.XML:      "application/xml",
		cmds.Text:     "text/plain",
	}
)

// NewResponeEmitter returns a new ResponseEmitter.
func NewResponseEmitter(w http.ResponseWriter, method string, req *cmds.Request) ResponseEmitter {
	encType := cmds.GetEncoding(req)

	var enc cmds.Encoder

	if _, ok := cmds.Encoders[encType]; ok {
		enc = cmds.Encoders[encType](req)(w)
	}

	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     enc,
		method:  method,
		req:     req,
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
	req     *cmds.Request

	l      sync.Mutex
	length uint64
	err    *cmdkit.Error

	streaming bool
	once      sync.Once
	method    string
}

func (re *responseEmitter) Emit(value interface{}) error {
	if single, ok := value.(cmds.Single); ok {
		value = single.Value
		defer re.closeWithError(nil)
	}

	// Initially this library allowed commands to return errors by sending an
	// error value along a stream. We removed that in favour of CloseWithError,
	// so we want to make sure we catch situations where some code still uses the
	// old error emitting semantics and _panic_ in those situations.
	debug.AssertNotError(value)

	// if we got a channel, instead emit values received on there.
	if ch, ok := value.(chan interface{}); ok {
		value = (<-chan interface{})(ch)
	}
	if ch, isChan := value.(<-chan interface{}); isChan {
		return cmds.EmitChan(re, ch)
	}

	re.once.Do(func() { re.preamble(value) })

	re.l.Lock()
	defer re.l.Unlock()

	var err error

	// return immediately if this is a head request
	if re.method == "HEAD" {
		return nil
	}

	if re.w == nil {
		return cmds.ErrClosedEmitter
	}

	// ignore those
	if value == nil {
		return nil
	}

	if err, ok := value.(cmdkit.Error); ok {
		panic("fixme: got a cmdkit.Error: " + err.Error())
	}

	if err, ok := value.(*cmdkit.Error); ok {
		panic("fixme: got a *cmdkit.Error: " + err.Error())
	}

	switch v := value.(type) {
	case io.Reader:
		err = flushCopy(re.w, v)
	default:
		err = re.enc.Encode(value)
	}

	if f, ok := re.w.(http.Flusher); ok {
		f.Flush()
	}

	if err != nil {
		log.Error(err)
	}

	return err
}

func (re *responseEmitter) SetLength(l uint64) {
	re.l.Lock()
	defer re.l.Unlock()

	h := re.w.Header()
	h.Set("X-Content-Length", strconv.FormatUint(l, 10))

	re.length = l
}

func (re *responseEmitter) Close() error {
	return re.CloseWithError(nil)
}

func (re *responseEmitter) CloseWithError(err error) error {
	re.l.Lock()
	defer re.l.Unlock()

	return re.closeWithError(err)
}

func (re *responseEmitter) closeWithError(err error) error {
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		// abort by sending an error trailer
		re.w.Header().Set(StreamErrHeader, err.Error())
	}

	// use preamble directly, we're already in critical section
	re.once.Do(func() { re.doPreamble(err) })

	return nil
}

// Flush the http connection
func (re *responseEmitter) Flush() {
	re.once.Do(func() { re.preamble(nil) })

	if flusher, ok := re.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (re *responseEmitter) preamble(value interface{}) {
	re.l.Lock()
	defer re.l.Unlock()

	re.doPreamble(value)
}

func (re *responseEmitter) doPreamble(value interface{}) {
	var (
		h      = re.w.Header()
		status = http.StatusOK
		mime   string
	)

	switch v := value.(type) {
	case io.Reader:
		// set streams output type to text to avoid issues with browsers rendering
		// html pages on priveleged api ports
		h.Set(streamHeader, "1")
		re.streaming = true

		mime = "text/plain"
	case cmds.Single:
		// don't set stream/channel header
	case *cmdkit.Error:
		err := v
		if err.Code == cmdkit.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
	case error:
		status = http.StatusInternalServerError
	default:
		h.Set(channelHeader, "1")
	}

	// Set up our potential trailer
	h.Set("Trailer", StreamErrHeader)

	if mime == "" {
		var ok bool

		// lookup mime type from map
		mime, ok = mimeTypes[re.encType]
		if !ok {
			// catch-all, set to text as default
			mime = "text/plain"
		}
	}

	h.Set(contentTypeHeader, mime)

	// set 'allowed' headers
	h.Set("Access-Control-Allow-Headers", AllowedExposedHeaders)
	// expose those headers
	h.Set("Access-Control-Expose-Headers", AllowedExposedHeaders)

	re.w.WriteHeader(status)
}

type responseWriterer interface {
	Lower() http.ResponseWriter
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	re.enc = enc(re.w)
}

func flushCopy(w io.Writer, r io.Reader) error {
	buf := make([]byte, 4096)
	f, ok := w.(http.Flusher)
	if !ok {
		_, err := io.Copy(w, r)
		return err
	}
	for {
		n, err := r.Read(buf)
		switch err {
		case io.EOF:
			if n <= 0 {
				return nil
			}
			// if data was returned alongside the EOF, pretend we didnt
			// get an EOF. The next read call should also EOF.
		case nil:
			// continue
		default:
			return err
		}

		nw, err := w.Write(buf[:n])
		if err != nil {
			return err
		}

		if nw != n {
			return fmt.Errorf("http write failed to write full amount: %d != %d", nw, n)
		}

		f.Flush()
	}
}
