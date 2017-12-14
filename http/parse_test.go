package http

import (
	"net/http"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestParse(t *testing.T) {
	root := &cmds.Command{
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

	r, err := http.NewRequest("GET", "/api/v0/block/put", nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err := Parse(r, root)
	if err != nil {
		t.Fatal(err)
	}
	pth := req.Path()
	if pth[0] != "block" || pth[1] != "put" || len(pth) != 2 {
		t.Errorf("incorrect path %v, expected %v", pth, []string{"block", "put"})
	}

	r, err = http.NewRequest("GET", "/api/v0/block/bla", nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err = Parse(r, root)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
