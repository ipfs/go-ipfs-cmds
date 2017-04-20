package cmds

import (
	"testing"
)

func TestReqLog(t *testing.T) {
	l := &ReqLog{}

	req1, err := NewEmptyRequest()
	if err != nil {
		t.Fatal(err)
	}

	req2, err := NewEmptyRequest()
	if err != nil {
		t.Fatal(err)
	}

	req1.Values()["foo"] = 42
	req2.Values()["bar"] = 23

	rle1 := l.Add(req1)
	rle2 := l.Add(req2)

	l.ClearInactive()

	if len(l.Report()) != 2 {
		t.Fatal("cleaned up too much")
	}

	rle1.Active = false

	l.ClearInactive()

	l.Finish(rle2)

	if len(l.Report()) != 1 {
		t.Fatal("cleaned up too much")
	}

	l.ClearInactive()

	if len(l.Report()) != 0 {
		t.Fatal("cleaned up too little")
	}

}
