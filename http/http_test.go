package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestHTTP(t *testing.T) {
	type testcase struct {
		path    []string
		v       interface{}
		sendErr error
		nextErr error
	}

	tcs := []testcase{
		{
			path: []string{"version"},
			v: &VersionOutput{
				Version: "0.1.2",
				Commit:  "c0mm17",
				Repo:    "4",
				System:  runtime.GOARCH + "/" + runtime.GOOS, //TODO: Precise version here
				Golang:  runtime.Version(),
			},
		},
		{
			path:    []string{"error"},
			sendErr: errors.New("an error occurred"),
		},
	}

	mkTest := func(tc testcase) func(*testing.T) {
		return func(t *testing.T) {
			srv := getTestServer(t, nil) // handler_test:/^func getTestServer/
			c := NewClient(srv.URL)
			req, err := cmds.NewRequest(context.Background(), tc.path, nil, nil, nil, cmdRoot)
			if err != nil {
				t.Fatal(err)
			}

			res, err := c.Send(req)
			if tc.sendErr != nil {
				if err == nil {
					t.Fatal("got nil error, expected:", tc.sendErr)
				} else if err.Error() != tc.sendErr.Error() {
					t.Fatalf("got error %q, expected %q", err, tc.sendErr)
				}

				return
			} else if err != nil {
				t.Fatal("unexpected error:", err)
			}

			v, err := res.Next()
			if tc.nextErr != nil {
				if err == nil {
					t.Fatal("got nil error, expected:", tc.nextErr)
				} else if err.Error() != tc.nextErr.Error() {
					t.Fatalf("got error %q, expected %q", err, tc.nextErr)
				}
			} else if err != nil {
				t.Fatal("unexpected error:", err)
			}

			if !reflect.DeepEqual(v, tc.v) {
				t.Errorf("expected value to be %v but got %v", tc.v, v)
			}

			_, err = res.Next()
			if tc.nextErr != nil {
				if err == nil {
					t.Fatal("got nil error, expected:", tc.nextErr)
				} else if err.Error() != tc.nextErr.Error() {
					t.Fatalf("got error %q, expected %q", err, tc.nextErr)
				}
			} else if err != io.EOF {
				t.Fatal("expected io.EOF error, got:", err)
			}
		}
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprint(i), mkTest(tc))
	}
}
