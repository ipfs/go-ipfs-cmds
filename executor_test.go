package cmds

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

var theError = errors.New("an error occurred")

var root = &Command{
	Subcommands: map[string]*Command{
		"test": &Command{
			Run: func(req *Request, re ResponseEmitter, env Environment) error {
				re.Emit(env)
				return nil
			},
		},
		"testError": &Command{
			Run: func(req *Request, re ResponseEmitter, env Environment) error {
				err := theError
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

func (e *env) Context() context.Context {
	return context.Background()
}

func TestExecutor(t *testing.T) {
	env := env(42)
	req, err := NewRequest(context.Background(), []string{"test"}, nil, nil, nil, root)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	re := NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req, Encoders[Text])

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
	var buf bytes.Buffer
	re := NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req, Encoders[Text])

	x := NewExecutor(root)
	x.Execute(req, re, &env)

	expected := "{\"Message\":\"an error occurred\",\"Code\":0,\"Type\":\"error\"}\n"
	if out := buf.String(); out != expected {
		t.Errorf("expected output \"%s\" but got %q", expected, out)
	}
}
