package cmds

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

var errGeneric = errors.New("an error occurred")

var root = &Command{
	Subcommands: map[string]*Command{
		"test": {
			Run: func(req *Request, re ResponseEmitter, env Environment) error {
				re.Emit(env)
				return nil
			},
		},
		"testError": {
			Run: func(req *Request, re ResponseEmitter, env Environment) error {
				err := errGeneric
				if err != nil {
					return err
				}
				re.Emit(env)
				return nil
			},
		},
	},
}

type wc struct {
	io.Writer
	io.Closer
}

type env int

func TestExecutor(t *testing.T) {
	env := env(42)
	req, err := NewRequest(context.Background(), []string{"test"}, nil, nil, nil, root)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	re, err := NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req)
	if err != nil {
		t.Fatal(err)
	}

	x := NewExecutor(root)
	x.Execute(req, re, &env)

	if out := buf.String(); out != "42\n" {
		t.Errorf("expected output \"42\" but got %q", out)
	}
}

func TestExecutorError(t *testing.T) {
	env := env(42)
	req, err := NewRequest(context.Background(), []string{"testError"}, nil, nil, nil, root)
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	x := NewExecutor(root)
	x.Execute(req, re, &env)

	_, err = res.Next()
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	expErr := "an error occurred"
	if err.Error() != expErr {
		t.Fatalf("expected error message %q but got: %s", expErr, err)
	}
}

func TestExecutorNotTyper(t *testing.T) {
	testCmd := &Command{
		Run: func(*Request, ResponseEmitter, Environment) error {
			return nil
		},
		PostRun: PostRunMap{
			CLI: func(response Response, emitter ResponseEmitter) error { return nil },
		},
	}
	testRoot := &Command{
		Subcommands: map[string]*Command{
			"test": testCmd,
		},
	}
	req, err := NewRequest(context.Background(), []string{"test"}, nil, nil, nil, testRoot)
	if err != nil {
		t.Fatal(err)
	}

	emitter, resp := NewChanResponsePair(req)

	x := NewExecutor(testRoot)

	err = x.Execute(req, emitter, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = resp.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF but got: %s", err)
	}
}

func TestExecutorPostRun(t *testing.T) {
	expectedValue := errors.New("postrun ran")
	testCmd := &Command{
		Run: func(*Request, ResponseEmitter, Environment) error {
			return nil
		},
		PostRun: PostRunMap{
			CLI: func(response Response, emitter ResponseEmitter) error {
				return expectedValue
			},
		},
	}
	testRoot := &Command{
		Subcommands: map[string]*Command{
			"test": testCmd,
		},
	}
	req, err := NewRequest(context.Background(), []string{"test"}, nil, nil, nil, testRoot)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("with", func(t *testing.T) {
		var (
			emitter, resp = NewChanResponsePair(req)
			x             = NewExecutor(testRoot)
		)
		emitter = cliMockEmitter{emitter}

		if err := x.Execute(req, emitter, nil); err != nil {
			t.Fatal(err)
		}

		_, err := resp.Next()
		if err != expectedValue {
			t.Fatalf("expected Response to return %s but got: %s",
				expectedValue, err)
		}
	})

	t.Run("without", func(t *testing.T) {
		testCmd.PostRun = nil

		var (
			emitter, resp = NewChanResponsePair(req)
			x             = NewExecutor(testRoot)
		)
		emitter = cliMockEmitter{emitter}
		if err := x.Execute(req, emitter, nil); err != nil {
			t.Fatal(err)
		}

		_, err := resp.Next()
		if err != io.EOF {
			t.Fatalf("expected EOF but got: %s", err)
		}
	})
}

type cliMockEmitter struct{ ResponseEmitter }

func (cliMockEmitter) Type() PostRunType { return CLI }
