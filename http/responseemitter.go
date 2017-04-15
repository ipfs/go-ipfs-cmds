package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"

	"github.com/ipfs/go-ipfs/repo/config"
)

var (
	HeadRequest = fmt.Errorf("HEAD request")
)

// NewResponeEmitter returns a new ResponseEmitter.
func NewResponseEmitter(w http.ResponseWriter, method string, req cmds.Request) ResponseEmitter {
	log.Debugf("entering NewResponseEmitter with w=%v, method=%s, req=%v", w, method, req)
	encType := cmds.GetEncoding(req)
	log.Debugf("got encoding %s", encType)

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
	req     cmds.Request

	length uint64
	err    *cmdsutil.Error

	once   sync.Once
	method string
}

func (re *responseEmitter) Emit(value interface{}) error {
	ch, isChan := value.(<-chan interface{})
	if !isChan {
		ch, isChan = value.(chan interface{})
	}

	if isChan {
		for value = range ch {
			err := re.Emit(value)
			if err != nil {
				return err
			}
		}
		return nil
	}

	var err error
	log.Debugf("Emit(%T) - %v", value, value)

	re.once.Do(func() { re.preamble(value) })

	if re.w == nil {
		return fmt.Errorf("connection already closed / custom - http.respem - TODO")
	}

	// ignore those
	if value == nil {
		return nil
	}

	// return immediately if this is a head request
	if re.method == "HEAD" {
		return nil
	}

	switch v := value.(type) {
	case io.Reader:
		err = flushCopy(re.w, v)
	case cmdsutil.Error:
		log.Debugf("Emit.switch - case cmdsutil.Error")
		re.w.Header().Set(StreamErrHeader, v.Error())
	case *cmdsutil.Error:
		log.Debugf("Emit.switch - case *cmdsutil.Error")
		log.Debug(re.w)
		log.Debug(re.w.Header())
		v.Error()
		re.w.Header().Set(StreamErrHeader, v.Error())
	default:
		err = re.enc.Encode(value)
		re.w.(http.Flusher).Flush()
	}

	return err
}

func (re *responseEmitter) SetLength(l uint64) {
	h := re.w.Header()
	h.Set("X-Content-Length", strconv.FormatUint(l, 10))

	re.length = l
}

func (re *responseEmitter) Close() error {
	// can't close HTTP connections
	return nil
}

func (re *responseEmitter) SetError(v interface{}, errType cmdsutil.ErrorType) error {
	log.Debugf("re.SetError(%v, %v)", v, errType)
	return re.Emit(&cmdsutil.Error{Message: fmt.Sprint(v), Code: errType})
}

// Flush the http connection
func (re *responseEmitter) Flush() {
	re.once.Do(func() { re.preamble(nil) })

	re.w.(http.Flusher).Flush()
}

func (re *responseEmitter) preamble(value interface{}) {
	log.Debugf("re.preamble, v=%#v", value)
	defer log.Debug("preemble done, headers: ", re.w.Header())

	h := re.w.Header()
	// Expose our agent to allow identification
	h.Set("Server", "go-ipfs/"+config.CurrentVersionNumber)

	status := http.StatusOK

	switch v := value.(type) {
	case *cmdsutil.Error:
		err := v

		if err.Code == cmdsutil.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}

		http.Error(re.w, err.Error(), status)
		re.w = nil

		log.Debug("sent error: ", err)

		return
	case io.Reader:
		// set streams output type to text to avoid issues with browsers rendering
		// html pages on priveleged api ports
		h.Set(streamHeader, "1")
	default:
		h.Set(channelHeader, "1")
	}

	log.Debugf("preamble status=%v", status)

	// Set up our potential trailer
	h.Set("Trailer", StreamErrHeader)

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

type responseWriterer interface {
	Lower() http.ResponseWriter
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	log.Debugf("SetEncoder called :( '%s'", re.encType)
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
	return nil
}
