package cmds

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type Foo struct {
	Bar int
}

type Bar struct {
	Foo string
}

type ValueError struct {
	Error error
	Value interface{}
}

type anyTestCase struct {
	Types   []interface{}
	JSON    string
	Decoded []ValueError
}

func TestMaybe(t *testing.T) {
	testcases := []anyTestCase{
		anyTestCase{
			Types: []interface{}{Foo{}, &Bar{}},
			JSON:  `{"Bar":2}{"Foo":"abc"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: Foo{2}},
				ValueError{Error: nil, Value: Bar{"abc"}},
			},
		},
	}

	for _, tc := range testcases {
		a := &Any{}

		for _, t := range tc.Types {
			a.Add(t)
		}

		r := strings.NewReader(tc.JSON)
		d := json.NewDecoder(r)

		var err error

		for _, dec := range tc.Decoded {
			err = d.Decode(a)
			if err != dec.Error {
				t.Fatalf("error is %v, expected %v", err, dec.Error)
			}

			if a.Interface() != dec.Value {
				t.Fatalf("value is %#v, expected %#v", a.Interface(), dec.Value)
			}
		}

		err = d.Decode(a)
		if err != io.EOF {
			t.Fatal("data left in decoder:", a.Interface())
		}
	}
}
