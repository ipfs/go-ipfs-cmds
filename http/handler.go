package http

import (
	"errors"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-loggables"
	cors "github.com/rs/cors"
)

var log = logging.Logger("cmds/http")

var (
	ErrNotFound           = errors.New("404 page not found")
	errApiVersionMismatch = errors.New("api version mismatch")
)

const (
	StreamErrHeader          = "X-Stream-Error"
	streamHeader             = "X-Stream-Output"
	channelHeader            = "X-Chunked-Output"
	extraContentLengthHeader = "X-Content-Length"
	uaHeader                 = "User-Agent"
	contentTypeHeader        = "Content-Type"
	contentDispHeader        = "Content-Disposition"
	transferEncodingHeader   = "Transfer-Encoding"
	originHeader             = "origin"

	applicationJson        = "application/json"
	applicationOctetStream = "application/octet-stream"
	plainText              = "text/plain"
)

func skipAPIHeader(h string) bool {
	switch h {
	case "Access-Control-Allow-Origin":
		return true
	case "Access-Control-Allow-Methods":
		return true
	case "Access-Control-Allow-Credentials":
		return true
	default:
		return false
	}
}

// the internal handler for the API
type handler struct {
	root *cmds.Command
	cfg  *ServerConfig
	env  interface{}
}

func NewHandler(env interface{}, root *cmds.Command, cfg *ServerConfig) http.Handler {
	if cfg == nil {
		panic("must provide a valid ServerConfig")
	}

	c := cors.New(*cfg.corsOpts)

	var h http.Handler

	h = &handler{
		env:  env,
		root: root,
		cfg:  cfg,
	}

	if cfg.APIPath != "" {
		h = newPrefixHandler(cfg.APIPath, h) // wrap with path prefix checker and trimmer
	}
	h = c.Handler(h) // wrap with CORS handler

	return h
}

type requestLogger interface {
	LogRequest(*cmds.Request) func()
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("incoming API request: ", r.URL)

	defer func() {
		if r := recover(); r != nil {
			log.Error("a panic has occurred in the commands handler!")
			log.Error(r)
			log.Errorf("stack trace:\n%s", debug.Stack())
		}
	}()

	ctx := r.Context()
	ctx = logging.ContextWithLoggable(ctx, loggables.Uuid("requestId"))

	if !allowOrigin(r, h.cfg) || !allowReferer(r, h.cfg) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		log.Warningf("API blocked request to %s. (possible CSRF)", r.URL)
		return
	}

	req, err := parseRequest(ctx, r, h.root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}

	if reqLogger, ok := h.env.(requestLogger); ok {
		done := reqLogger.LogRequest(req)
		defer done()
	}

	// set user's headers first.
	for k, v := range h.cfg.Headers {
		if !skipAPIHeader(k) {
			w.Header()[k] = v
		}
	}

	re := NewResponseEmitter(w, r.Method, req)
	defer re.Close()

	// call the command
	err = h.root.Call(req, re, h.env)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	d := re.(Doner)
	select {
	case <-d.Done():
	case <-req.Context.Done():
		log.Errorf("waiting for http.Responseemitter to close but then %s", req.Context.Err())
		// too late to send an error, just return
	}

	return
}

func sanitizedErrStr(err error) string {
	s := err.Error()
	s = strings.Split(s, "\n")[0]
	s = strings.Split(s, "\r")[0]
	return s
}
