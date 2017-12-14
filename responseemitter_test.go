package cmds

import (
	"io"
	"testing"
)

func TestCopy(t *testing.T) {
	req, err := NewRequest(nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	re1, res1 := NewChanResponsePair(req)
	re2, res2 := NewChanResponsePair(req)

	go func() {
		err := Copy(re2, res1)
		if err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		err := re1.Emit("test")
		if err != nil {
			t.Fatal(err)
		}

		err = re1.Close()
		if err != nil {
			t.Fatal(err)
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
