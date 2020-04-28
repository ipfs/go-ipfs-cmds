package cmds

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestCopy(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re1, res1 := NewChanResponsePair(req)
	re2, res2 := NewChanResponsePair(req)

	go func() {
		err := Copy(re2, res1)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	go func() {
		err := re1.Emit("test")
		if err != nil {
			t.Error(err)
			return
		}

		err = re1.Close()
		if err != nil {
			t.Error(err)
			return
		}
	}()

	v, err := res2.Next()
	if err != nil {
		t.Fatal(err)
	}

	str := v.(string)
	if str != "test" {
		t.Fatalf("expected string %#v but got %#v", "test", str)
	}

	_, err = res2.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF but got err=%v", err)
	}
}

func TestCopyError(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	fooErr := fmt.Errorf("foo")

	re1, res1 := NewChanResponsePair(req)
	re2, res2 := NewChanResponsePair(req)

	go func() {
		err := Copy(re2, res1)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	go func() {
		err := re1.Emit("test")
		if err != nil {
			t.Error(err)
			return
		}

		err = re1.CloseWithError(fooErr)
		if err != nil {
			t.Error(err)
			return
		}
	}()

	v, err := res2.Next()
	if err != nil {
		t.Fatal(err)
	}

	str := v.(string)
	if str != "test" {
		t.Fatalf("expected string %#v but got %#v", "test", str)
	}

	_, err = res2.Next()
	if err != fooErr {
		t.Fatalf("expected fooErr but got err=%v", err)
	}
}

func TestError(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	go func() {
		err := re.Emit("value1")
		if err != nil {
			t.Error(err)
			return
		}

		err = re.Emit("value2")
		if err != nil {
			t.Error(err)
			return
		}

		err = re.CloseWithError(&Error{Message: "foo"})
		if err != nil {
			t.Error(err)
			return
		}
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "value1" {
		t.Errorf("expected string %#v but got %#v", "value1", v)
	}

	v, err = res.Next()
	if err != nil {
		t.Error(err)
	}
	if v.(string) != "value2" {
		t.Errorf("expected string %#v but got %#v", "value1", v)
	}

	v, err = res.Next()
	if v != nil {
		t.Errorf("expected nil value, got %#v", v)

	}
	e, ok := err.(*Error)
	if !ok {
		t.Errorf("expected error to be %T, got %T", e, v)
	} else {
		expMsg := "foo"
		if e.Message != expMsg {
			t.Errorf("expected error message to be %q, got %q", expMsg, e.Message)
		}

		if e.Code != 0 {
			t.Errorf("expected error code 0(ErrNormal), got %v", e.Code)
		}
	}
}
