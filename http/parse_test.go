package http

import (
	"net/http"
	"reflect"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

var root = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"block": &cmds.Command{
			Subcommands: map[string]*cmds.Command{
				"put": &cmds.Command{
					Run: func(req cmds.Request, resp cmds.ResponseEmitter) {
						defer resp.Close()
						resp.Emit("done")
					},
				},
			},
		},
	},
}

func TestParse(t *testing.T) {
	type testcase struct {
		path     string
		parseErr error
		reqPath  []string
	}

	tcs := []testcase{
		{path: "block/put", parseErr: nil, reqPath: []string{"block", "put"}},
		{path: "block/bla", parseErr: ErrNotFound, reqPath: nil},
		{path: "block/put/foo", parseErr: ErrNotFound, reqPath: nil},
	}

	for _, tc := range tcs {
		r, err := http.NewRequest("GET", "/api/v0/"+tc.path, nil)
		if err != nil {
			t.Error(err)
			continue
		}
		req, err := Parse(r, root)
		if err != tc.parseErr {
			t.Errorf("expected parse error %q, got %q", tc.parseErr, err)
		}
		if err != nil {
			continue
		}
		pth := req.Path()
		if !reflect.DeepEqual(pth, tc.reqPath) {
			t.Errorf("incorrect path %v, expected %v", pth, []string{"block", "put"})
		}
	}
}
