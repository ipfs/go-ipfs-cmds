package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// TestCustomExposeHeadersPreserved verifies that when a caller sets
// Access-Control-Expose-Headers via ServerConfig.Headers (the way kubo's
// addHeadersFromConfig does for user-configured values), the response
// includes both the caller-provided values AND the default stream-related
// headers, instead of overwriting them.
//
// See https://github.com/ipfs/kubo/issues/9586
func TestCustomExposeHeadersPreserved(t *testing.T) {
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"version": {
				Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
					return re.Emit("0.1.2")
				},
			},
		},
	}

	cfg := NewServerConfig()
	cfg.SetAllowedOrigins("*")
	cfg.SetAllowedMethods("GET", "POST")
	cfg.AllowGet = true

	// Simulate user-configured Access-Control-Expose-Headers, the way
	// kubo's addHeadersFromConfig copies non-CORS-library headers into
	// ServerConfig.Headers.
	cfg.Headers = map[string][]string{
		http.CanonicalHeaderKey("Access-Control-Expose-Headers"): {"X-Custom-Expose"},
	}

	srv := httptest.NewServer(NewHandler(nil, root, cfg))
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	expose := res.Header.Get("Access-Control-Expose-Headers")
	t.Logf("Access-Control-Expose-Headers: %s", expose)

	// The user-configured value must be present.
	if !strings.Contains(expose, "X-Custom-Expose") {
		t.Errorf("Access-Control-Expose-Headers missing user-configured 'X-Custom-Expose'; got: %s", expose)
	}

	// The default stream-related headers must also be present.
	for _, def := range AllowedExposedHeadersArr {
		if !strings.Contains(expose, def) {
			t.Errorf("Access-Control-Expose-Headers missing default '%s'; got: %s", def, expose)
		}
	}

	// Access-Control-Allow-Headers must NOT be set by doPreamble. It is
	// managed by the CORS middleware (rs/cors) based on AllowedHeaders.
	if allowHeaders := res.Header.Get("Access-Control-Allow-Headers"); strings.Contains(allowHeaders, streamHeader) {
		t.Errorf("Access-Control-Allow-Headers should not contain stream output headers; got: %s", allowHeaders)
	}
}
