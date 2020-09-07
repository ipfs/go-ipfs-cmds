package http

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestErrors(t *testing.T) {
	type testcase struct {
		opts       cmds.OptMap
		path       []string
		bodyStr    string
		status     string
		errTrailer string
	}

	tcs := []testcase{
		{
			path: []string{"version"},
			bodyStr: `{` +
				`"Version":"0.1.2",` +
				`"Commit":"c0mm17",` +
				`"Repo":"4",` +
				`"System":"` + runtime.GOARCH + "/" + runtime.GOOS + `",` +
				`"Golang":"` + runtime.Version() + `"}` + "\n",
			status: "200 OK",
		},

		// TODO this error should be sent as a value, because it is non-200
		{
			path:    []string{"error"},
			status:  "500 Internal Server Error",
			bodyStr: `{"Message":"an error occurred","Code":0,"Type":"error"}` + "\n",
		},

		{
			path:       []string{"lateerror"},
			status:     "200 OK",
			bodyStr:    `"some value"` + "\n",
			errTrailer: "an error occurred",
		},

		{
			path: []string{"encode"},
			opts: cmds.OptMap{
				cmds.EncLong: cmds.Text,
			},
			status:  "500 Internal Server Error",
			bodyStr: "an error occurred",
		},

		{
			path: []string{"lateencode"},
			opts: cmds.OptMap{
				cmds.EncLong: cmds.Text,
			},
			status:     "200 OK",
			bodyStr:    "hello\n",
			errTrailer: "an error occurred",
		},

		{
			path: []string{"protoencode"},
			opts: cmds.OptMap{
				cmds.EncLong: cmds.Protobuf,
			},
			status:  "500 Internal Server Error",
			bodyStr: `{"Message":"an error occurred","Code":0,"Type":"error"}` + "\n",
		},

		{
			path: []string{"protolateencode"},
			opts: cmds.OptMap{
				cmds.EncLong: cmds.Protobuf,
			},
			status:     "200 OK",
			bodyStr:    "hello\n",
			errTrailer: "an error occurred",
		},

		{
			// bad encoding
			path: []string{"error"},
			opts: cmds.OptMap{
				cmds.EncLong: "foobar",
			},
			status:  "400 Bad Request",
			bodyStr: "invalid encoding: foobar\n",
		},

		{
			path:    []string{"doubleclose"},
			status:  "200 OK",
			bodyStr: `"some value"` + "\n",
		},

		{
			path:    []string{"single"},
			status:  "200 OK",
			bodyStr: `"some value"` + "\n",
		},

		{
			path:    []string{"reader"},
			status:  "200 OK",
			bodyStr: "the reader call returns a reader.",
		},
	}

	mkTest := func(tc testcase) func(*testing.T) {
		return func(t *testing.T) {
			_, srv := getTestServer(t, nil, false) // handler_test:/^func getTestServer/
			c := NewClient(srv.URL)
			req, err := cmds.NewRequest(context.Background(), tc.path, tc.opts, nil, nil, cmdRoot)
			if err != nil {
				t.Fatal(err)
			}

			httpReq, err := c.(*client).toHTTPRequest(req)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			httpClient := http.DefaultClient

			res, err := httpClient.Do(httpReq)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			if res.Status != tc.status {
				t.Errorf("expected status %v, got %v", tc.status, res.Status)
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal("err reading response body", err)
			}

			if bodyStr := string(body); bodyStr != tc.bodyStr {
				t.Errorf("expected body string \n\n%v\n\n, got\n\n%v", tc.bodyStr, bodyStr)
			}

			if errTrailer := res.Trailer.Get(StreamErrHeader); errTrailer != tc.errTrailer {
				t.Errorf("expected error header %q, got %q", tc.errTrailer, errTrailer)
			}
		}
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d-%s", i, strings.Join(tc.path, "/")), mkTest(tc))
	}
}

func TestUnhandledMethod(t *testing.T) {
	tc := httpTestCase{
		Method:   "GET",
		AllowGet: false,
		Code:     http.StatusMethodNotAllowed,
		ResHeaders: map[string]string{
			"Allow": "OPTIONS, POST",
		},
	}
	tc.test(t)
}

func TestDisallowedUserAgents(t *testing.T) {
	tcs := []httpTestCase{
		{
			// Block Mozilla* browsers that do not provide origins.
			Method:   "POST",
			AllowGet: false,
			Code:     http.StatusForbidden,
			ReqHeaders: map[string]string{
				"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:10.0) Gecko/20100101 Firefox/10.0",
			},
		},
		{
			// Do not block on GETs
			Method:   "GET",
			AllowGet: true,
			Code:     http.StatusOK,
			ReqHeaders: map[string]string{
				"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:10.0) Gecko/20100101 Firefox/10.0",
			},
		},
		{
			// Do not block a Mozilla* browser that provides an
			// allowed Origin
			Method:       "POST",
			AllowGet:     false,
			AllowOrigins: []string{"*"},
			Origin:       "null",
			Code:         http.StatusOK,
			ReqHeaders: map[string]string{
				"User-Agent": "Mozilla/5.0 (Linux; U; Android 4.1.1; en-gb; Build/KLP) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Safari/534.30",
			},
		},
		{
			// Do not block the Electron Renderer process
			Method:   "POST",
			AllowGet: false,
			Code:     http.StatusOK,
			ReqHeaders: map[string]string{
				"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.104 Electron/9.0.4 Safari/537.36",
			},
		},
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}
