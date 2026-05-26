package http

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// TestRequestHeadersForwarded checks that the HTTP transport forwards
// inbound request headers to command handlers via [cmds.Request.Headers].
// Guards the contract that handlers can read request-scoped metadata
// (e.g. correlation ids) without needing custom middleware.
func TestRequestHeadersForwarded(t *testing.T) {
	const (
		marker      = "X-Test-Marker"
		markerValue = "marker-42"
	)

	type captured struct {
		headers map[string][]string
	}
	captureCh := make(chan captured, 1)

	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"echo-headers": {
				Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
					var c captured
					if req.Headers != nil {
						c.headers = map[string][]string(req.Headers.Clone())
					}
					captureCh <- c
					return re.Emit("ok")
				},
			},
		},
	}

	cfg := NewServerConfig()
	cfg.SetAllowedOrigins("*")
	cfg.SetAllowedMethods("POST")
	srv := httptest.NewServer(NewHandler(nil, root, cfg))
	defer srv.Close()

	c := NewClient(srv.URL, ClientWithHeader(marker, markerValue))

	req, err := cmds.NewRequest(context.Background(), []string{"echo-headers"}, nil, nil, nil, root)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	emitter, response := cmds.NewChanResponsePair(req)
	go func() {
		_ = c.Execute(req, emitter, nil)
	}()

	// Drain the response so the server-side handler runs to completion.
	// This test asserts on captureCh, not on response errors.
	_, _ = response.Next()

	got, ok := <-captureCh
	if !ok {
		t.Fatal("handler did not run")
	}
	if got.headers == nil {
		t.Fatal("Request.Headers was nil; expected populated for HTTP-dispatched requests")
	}
	want := []string{markerValue}
	if !reflect.DeepEqual(got.headers[marker], want) {
		t.Fatalf("Request.Headers[%q] = %v, want %v", marker, got.headers[marker], want)
	}
}

// TestLocalRequestHeadersNil checks that the local in-process executor
// leaves Request.Headers nil. A nil http.Header is the "no HTTP transport"
// sentinel; handlers can call .Get on it without panicking.
func TestLocalRequestHeadersNil(t *testing.T) {
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"check": {
				Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
					if req.Headers != nil {
						t.Errorf("Request.Headers should be nil for local dispatch; got %v", req.Headers)
					}
					if got := req.Headers.Get("X-Anything"); got != "" {
						t.Errorf("http.Header.Get on nil should return empty; got %q", got)
					}
					return re.Emit("ok")
				},
			},
		},
	}

	req, err := cmds.NewRequest(context.Background(), []string{"check"}, nil, nil, nil, root)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if req.Headers != nil {
		t.Fatalf("fresh local Request should have nil Headers; got %v", req.Headers)
	}

	exec := cmds.NewExecutor(root)
	emitter, response := cmds.NewChanResponsePair(req)
	go func() {
		_ = exec.Execute(req, emitter, nil)
	}()
	if _, err := response.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
}
