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
	log.Debug("request options: ", req.Options())

	var (
		encType = cmds.EncodingType(cmds.Undefined)
		encStr  = string(cmds.Undefined)
		ok      = false
		opts    = req.Options()
	)

	// try EncShort
	encSource := "short"
	encIface := opts[cmdsutil.EncShort]

	// if that didn't work, try EncLong
	if encIface == nil {
		encSource = "long"
		encIface = opts[cmdsutil.EncLong]
	}

	// try casting
	if encIface != nil {
		encStr, ok = encIface.(string)
	}

	log.Debug("req encType:", encSource, encStr, ok)

	// if casting worked, convert to EncodingType
	if ok {
		encType = cmds.EncodingType(encStr)
	}

	// in case of error, use default
	if !ok || encType == cmds.Undefined {
		encSource = "default"
		encType = cmds.JSON
	}

	log.Debug("chose encoding ", encType, " from source ", encSource)

	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     cmds.Encoders[encType](req)(w),
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

	emitted     bool
	emittedLock sync.Mutex
	method      string

	tees []cmds.ResponseEmitter
}

func (re *responseEmitter) Emit(value interface{}) error {
	var err error

	re.emittedLock.Lock()
	if !re.emitted {
		re.emitted = true
		re.preamble(value)
	}
	re.emittedLock.Unlock()

	go func() {
		for _, re_ := range re.tees {
			re_.Emit(value)
		}
	}()

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
		_, err = io.Copy(re.w, v)
	case cmdsutil.Error, *cmdsutil.Error:
		// nothing
	default:
		err = re.enc.Encode(value)
	}

	return err
}

func (re *responseEmitter) SetLength(l uint64) {
	h := re.w.Header()
	h.Set("X-Content-Length", strconv.FormatUint(l, 10))

	re.length = l

	for _, re_ := range re.tees {
		re_.SetLength(l)
	}
}

func (re *responseEmitter) Close() error {
	// can't close HTTP connections
	return nil
}

func (re *responseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	re.Emit(&cmdsutil.Error{Message: fmt.Sprint(err), Code: code})

	for _, re_ := range re.tees {
		re_.SetError(err, code)
	}
}

// Flush the http connection
func (re *responseEmitter) Flush() {
	if !re.emitted {
		re.emitted = true

		// setting this to nil means that it sends channel/chunked-encoding headers
		re.preamble(nil)
	}

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

func (re *responseEmitter) Tee(re_ cmds.ResponseEmitter) {
	re.tees = append(re.tees, re_)

	if re.emitted {
		re_.SetLength(re.length)
	}

	if re.err != nil {
		re_.SetError(re.err.Message, re.err.Code)
	}
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	log.Debugf("SetEncoder called :( '%s'", re.encType)
	re.enc = enc(re.w)
}
