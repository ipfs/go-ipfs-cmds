package http

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func assertHeaders(t *testing.T, resHeaders http.Header, reqHeaders map[string]string) {
	t.Helper()
	t.Logf("headers: %v", resHeaders)
	for name, value := range reqHeaders {
		header := resHeaders[http.CanonicalHeaderKey(name)]
		switch len(header) {
		case 0:
			if value != "" {
				t.Errorf("expected a header for %s", name)
			}
		case 1:
			if header[0] != value {
				t.Errorf("Invalid header '%s', wanted '%s', got '%s'", name, value, header[0])
			}
		default:
			values := strings.Split(value, ",")
			set := make(map[string]bool, len(values))
			for _, v := range values {
				set[strings.Trim(v, " ")] = true
			}
			for _, got := range header {
				if !set[got] {
					t.Errorf("found unexpected value %s in header %s", got, name)
					continue
				}
				delete(set, got)
			}
			for missing := range set {
				t.Errorf("missing value %s in header %s", missing, name)
			}
		}
	}
}

func assertStatus(t *testing.T, actual, expected int) {
	if actual != expected {
		t.Errorf("Expected status: %d got: %d", expected, actual)
	}
}

func originCfg(origins []string) *ServerConfig {
	cfg := NewServerConfig()
	cfg.SetAllowedOrigins(origins...)
	cfg.SetAllowedMethods("GET", "PUT", "POST")
	cfg.AllowGet = true
	return cfg
}

var defaultOrigins = []string{
	"http://localhost",
	"http://127.0.0.1",
	"https://localhost",
	"https://127.0.0.1",
}

type httpTestCase struct {
	Method       string
	Path         string
	Code         int
	Origin       string
	Referer      string
	AllowOrigins []string
	AllowGet     bool
	ReqHeaders   map[string]string
	ResHeaders   map[string]string
}

func (tc *httpTestCase) test(t *testing.T) {
	t.Helper()
	// defaults
	method := tc.Method
	if method == "" {
		method = "GET"
	}

	path := tc.Path
	if path == "" {
		path = "/version"
	}

	expectCode := tc.Code
	if expectCode == 0 {
		expectCode = 200
	}

	// request
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Error(err)
		return
	}

	for k, v := range tc.ReqHeaders {
		req.Header.Add(k, v)
	}
	if tc.Origin != "" {
		req.Header.Add("Origin", tc.Origin)
	}
	if tc.Referer != "" {
		req.Header.Add("Referer", tc.Referer)
	}

	// server
	_, server := getTestServer(t, tc.AllowOrigins, tc.AllowGet)
	if server == nil {
		return
	}
	defer server.Close()

	req.URL, err = url.Parse(server.URL + path)
	if err != nil {
		t.Error(err)
		return
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}

	// checks
	t.Log("GET", server.URL+path, req.Header, res.Header)
	assertHeaders(t, res.Header, tc.ResHeaders)
	assertStatus(t, res.StatusCode, expectCode)
}

func TestDisallowedOrigins(t *testing.T) {
	gtc := func(origin string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       origin,
			AllowOrigins: allowedOrigins,
			AllowGet:     true,
			ResHeaders: map[string]string{
				ACAOrigin:                       "",
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": "",
			},
			Code: http.StatusForbidden,
		}
	}

	tcs := []httpTestCase{
		gtc("http://barbaz.com", nil),
		gtc("http://barbaz.com", []string{"http://localhost"}),
		gtc("http://127.0.0.1", []string{"http://localhost"}),
		gtc("http://localhost", []string{"http://127.0.0.1"}),
		gtc("http://127.0.0.1:1234", nil),
		gtc("http://localhost:1234", nil),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestAllowedOrigins(t *testing.T) {
	gtc := func(origin string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       origin,
			AllowOrigins: allowedOrigins,
			AllowGet:     true,
			ResHeaders: map[string]string{
				ACAOrigin:                       origin,
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": AllowedExposedHeaders,
			},
			Code: http.StatusOK,
		}
	}

	tcs := []httpTestCase{
		gtc("http://barbaz.com", []string{"http://barbaz.com", "http://localhost"}),
		gtc("http://localhost", []string{"http://barbaz.com", "http://localhost"}),
		gtc("http://localhost", nil),
		gtc("http://127.0.0.1", nil),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestWildcardOrigin(t *testing.T) {
	gtc := func(origin string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       origin,
			AllowGet:     true,
			AllowOrigins: allowedOrigins,
			ResHeaders: map[string]string{
				ACAOrigin:                       "*",
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": AllowedExposedHeaders,
			},
			Code: http.StatusOK,
		}
	}

	tcs := []httpTestCase{
		gtc("http://barbaz.com", []string{"*"}),
		gtc("http://barbaz.com", []string{"http://localhost", "*"}),
		gtc("http://127.0.0.1", []string{"http://localhost", "*"}),
		gtc("http://localhost", []string{"http://127.0.0.1", "*"}),
		gtc("http://127.0.0.1", []string{"*"}),
		gtc("http://localhost", []string{"*"}),
		gtc("http://127.0.0.1:1234", []string{"*"}),
		gtc("http://localhost:1234", []string{"*"}),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestDisallowedReferer(t *testing.T) {
	gtc := func(referer string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       "http://localhost",
			Referer:      referer,
			AllowGet:     true,
			AllowOrigins: allowedOrigins,
			ResHeaders: map[string]string{
				ACAOrigin:                       "http://localhost",
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": "",
			},
			Code: http.StatusForbidden,
		}
	}

	tcs := []httpTestCase{
		gtc("http://foobar.com", nil),
		gtc("http://localhost:1234", nil),
		gtc("http://127.0.0.1:1234", nil),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestAllowedReferer(t *testing.T) {
	gtc := func(referer string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       "http://localhost",
			AllowOrigins: allowedOrigins,
			AllowGet:     true,
			ResHeaders: map[string]string{
				ACAOrigin:                       "http://localhost",
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": AllowedExposedHeaders,
			},
			Code: http.StatusOK,
		}
	}

	tcs := []httpTestCase{
		gtc("http://barbaz.com", []string{"http://barbaz.com", "http://localhost"}),
		gtc("http://localhost", []string{"http://barbaz.com", "http://localhost"}),
		gtc("http://localhost", nil),
		gtc("http://127.0.0.1", nil),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestWildcardReferer(t *testing.T) {
	gtc := func(origin string, allowedOrigins []string) httpTestCase {
		return httpTestCase{
			Origin:       origin,
			AllowOrigins: allowedOrigins,
			AllowGet:     true,
			ResHeaders: map[string]string{
				ACAOrigin:                       "*",
				ACAMethods:                      "",
				ACACredentials:                  "",
				"Access-Control-Max-Age":        "",
				"Access-Control-Expose-Headers": AllowedExposedHeaders,
			},
			Code: http.StatusOK,
		}
	}

	tcs := []httpTestCase{
		gtc("http://barbaz.com", []string{"*"}),
		gtc("http://barbaz.com", []string{"http://localhost", "*"}),
		gtc("http://127.0.0.1", []string{"http://localhost", "*"}),
		gtc("http://localhost", []string{"http://127.0.0.1", "*"}),
		gtc("http://127.0.0.1", []string{"*"}),
		gtc("http://localhost", []string{"*"}),
		gtc("http://127.0.0.1:1234", []string{"*"}),
		gtc("http://localhost:1234", []string{"*"}),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestAllowedMethod(t *testing.T) {
	gtc := func(method string, ok bool) httpTestCase {
		code := http.StatusOK
		hdrs := map[string]string{
			ACAOrigin:                       "*",
			ACAMethods:                      method,
			ACACredentials:                  "",
			"Access-Control-Max-Age":        "",
			"Access-Control-Expose-Headers": "",
		}

		if !ok {
			hdrs[ACAOrigin] = ""
			hdrs[ACAMethods] = ""
		}

		return httpTestCase{
			Method:       "OPTIONS",
			Origin:       "http://localhost",
			AllowOrigins: []string{"*"},
			ReqHeaders: map[string]string{
				"Access-Control-Request-Method": method,
			},
			ResHeaders: hdrs,
			Code:       code,
		}
	}

	tcs := []httpTestCase{
		gtc("PUT", true),
		gtc("GET", true),
		gtc("FOOBAR", false),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

func TestEncoding(t *testing.T) {
	gtc := func(enc, contentType string) httpTestCase {
		code := http.StatusOK
		hdrs := map[string]string{
			contentTypeHeader: contentType,
		}

		path := fmt.Sprintf("/version?%v=%v", cmds.EncShort, enc)

		return httpTestCase{
			Method:       "GET",
			Path:         path,
			AllowGet:     true,
			Origin:       "http://localhost",
			AllowOrigins: []string{"*"},
			ReqHeaders: map[string]string{
				"Access-Control-Request-Method": "GET",
			},
			ResHeaders: hdrs,
			Code:       code,
		}
	}

	tcs := []httpTestCase{
		gtc(cmds.JSON, applicationJSON),
		gtc(cmds.XML, "application/xml"),
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}
