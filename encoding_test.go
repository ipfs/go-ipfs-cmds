package cmds

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

type fooTestObj struct {
	Good bool
}

type barTestObj struct {
	Bad bool
}

func TestMakeTypedEncoder(t *testing.T) {
	expErr := fmt.Errorf("command fooTestObj failed")
	f := MakeTypedEncoder(func(req *Request, w io.Writer, v *fooTestObj) error {
		if v.Good {
			return nil
		}
		return expErr
	})

	req := &Request{}

	encoderFunc := f(req)

	buf := new(bytes.Buffer)
	encoder := encoderFunc(buf)

	if err := encoder.Encode(&fooTestObj{true}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Encode(&fooTestObj{false}); err != expErr {
		t.Fatal("expected: ", expErr)
	}
}

func TestMakeTypedEncoderByValue(t *testing.T) {
	expErr := fmt.Errorf("command fooTestObj failed")
	f := MakeTypedEncoder(func(req *Request, w io.Writer, v fooTestObj) error {
		if v.Good {
			return nil
		}
		return expErr
	})

	req := &Request{}

	encoderFunc := f(req)

	buf := new(bytes.Buffer)
	encoder := encoderFunc(buf)

	if err := encoder.Encode(&fooTestObj{true}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Encode(&fooTestObj{false}); err != expErr {
		t.Fatal("expected: ", expErr)
	}
}

func TestMakeTypedEncoderByPointer(t *testing.T) {
	expErr := fmt.Errorf("command fooTestObj failed")
	f := MakeTypedEncoder(func(req *Request, w io.Writer, v *fooTestObj) error {
		if v.Good {
			return nil
		}
		return expErr
	})

	req := &Request{}

	encoderFunc := f(req)

	buf := new(bytes.Buffer)
	encoder := encoderFunc(buf)

	if err := encoder.Encode(fooTestObj{true}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Encode(fooTestObj{false}); err != expErr {
		t.Fatal("expected: ", expErr)
	}
}

func TestMakeTypedEncoderArrays(t *testing.T) {
	f := MakeTypedEncoder(func(req *Request, w io.Writer, v []fooTestObj) error {
		if len(v) != 2 {
			return fmt.Errorf("bad")
		}
		return nil
	})

	req := &Request{}

	encoderFunc := f(req)

	buf := new(bytes.Buffer)
	encoder := encoderFunc(buf)

	if err := encoder.Encode([]fooTestObj{{true}, {false}}); err != nil {
		t.Fatal(err)
	}
}

func TestMakeMultiEncoder(t *testing.T) {
	expErrFoo := fmt.Errorf("command fooTestObj failed")
	expErrBar := fmt.Errorf("command barTestObj failed")

	f := MakeMultiEncoder(
		&fooTestObj{}, MakeTypedEncoder(func(req *Request, w io.Writer, v *fooTestObj) error {
			if v.Good {
				return nil
			}
			return expErrFoo
		}),
		&barTestObj{}, MakeTypedEncoder(func(req *Request, w io.Writer, v *barTestObj) error {
			if !v.Bad {
				return nil
			}
			return expErrBar
		}))

	req := &Request{}

	encoderFunc := f(req)

	buf := new(bytes.Buffer)
	encoder := encoderFunc(buf)

	if err := encoder.Encode(&fooTestObj{true}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Encode(&fooTestObj{false}); err != expErrFoo {
		t.Fatal("expected: ", expErrFoo)
	}

	if err := encoder.Encode(&barTestObj{true}); err != expErrBar {
		t.Fatal("expected: ", expErrBar)
	}

	if err := encoder.Encode(&barTestObj{false}); err != nil {
		t.Fatal(err)
	}
}
