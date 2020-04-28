package cli

import (
	"bytes"
	"fmt"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

type tcCloseWithError struct {
	stdout, stderr     *bytes.Buffer
	exStdout, exStderr string
	exExit             int
	f                  func(re ResponseEmitter, t *testing.T)
}

func (tc tcCloseWithError) Run(t *testing.T) {
	req := &cmds.Request{}
	cmdsre, err := NewResponseEmitter(tc.stdout, tc.stderr, req)
	if err != nil {
		t.Fatal(err)
	}

	re := cmdsre.(ResponseEmitter)

	tc.f(re, t)

	if re.Status() != tc.exExit {
		t.Fatalf("expected exit code %d, got %d", tc.exExit, re.Status())
	}

	if tc.stdout.String() != tc.exStdout {
		t.Fatalf(`expected stdout string "%s" but got "%s"`, tc.exStdout, tc.stdout.String())
	}

	if tc.stderr.String() != tc.exStderr {
		t.Fatalf(`expected stderr string "%s" but got "%s"`, tc.exStderr, tc.stderr.String())
	}

	t.Logf("stdout:\n---\n%s---\n", tc.stdout.Bytes())
	t.Logf("stderr:\n---\n%s---\n", tc.stderr.Bytes())
}

func TestCloseWithError(t *testing.T) {
	tcs := []tcCloseWithError{
		tcCloseWithError{
			stdout:   bytes.NewBuffer(nil),
			stderr:   bytes.NewBuffer(nil),
			exStdout: "a\n",
			exStderr: "Error: some error\n",
			exExit:   1,
			f: func(re ResponseEmitter, t *testing.T) {
				re.Emit("a")
				re.CloseWithError(fmt.Errorf("some error"))
				re.Emit("b")
			},
		},
		tcCloseWithError{
			stdout:   bytes.NewBuffer(nil),
			stderr:   bytes.NewBuffer(nil),
			exStdout: "a\n",
			exStderr: "Error: some error\n",
			exExit:   1,
			f: func(re ResponseEmitter, t *testing.T) {
				re.Emit("a")

				err := re.CloseWithError(fmt.Errorf("some error"))
				if err != nil {
					t.Fatal("unexpected error:", err)
				}

				err = re.Close()
				if err != cmds.ErrClosingClosedEmitter {
					t.Fatal("expected double close error, got:", err)
				}
			},
		},
	}

	for i, tc := range tcs {
		t.Log(i)
		tc.Run(t)
	}
}
