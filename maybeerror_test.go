package cmds

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs-cmdkit"
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
	Value   interface{}
	JSON    string
	Decoded []ValueError
}

func TestMaybeError(t *testing.T) {
	testcases := []anyTestCase{
		anyTestCase{
			Value: Foo{},
			JSON:  `{"Bar":23}{"Bar":42}{"Message":"some error", "Type": "error"}`,
			Decoded: []ValueError{
				ValueError{Error: nil, Value: &Foo{23}},
				ValueError{Error: nil, Value: &Foo{42}},
				ValueError{Error: nil, Value: cmdkit.Error{Message: "some error", Code: 0}},
			},
		},
	}

	for _, tc := range testcases {
		m := &MaybeError{Value: tc.Value}

		r := strings.NewReader(tc.JSON)
		d := json.NewDecoder(r)

		var err error

		for _, dec := range tc.Decoded {
			err = d.Decode(m)
			if err != dec.Error {
				t.Fatalf("error is %v, expected %v", err, dec.Error)
			}

			rx := m.Get()
			rxIsPtr := reflect.TypeOf(rx).Kind() == reflect.Ptr

			ex := dec.Value
			exIsPtr := reflect.TypeOf(ex).Kind() == reflect.Ptr

			if rxIsPtr != exIsPtr {
				t.Fatalf("value is %#v, expected %#v", m.Get(), dec.Value)
			}

			if rxIsPtr {
				rx = reflect.ValueOf(rx).Elem().Interface()
				ex = reflect.ValueOf(ex).Elem().Interface()
			}

			if rx != ex {
				t.Fatalf("value is %#v, expected %#v", m.Get(), dec.Value)
			}
		}

		err = d.Decode(m)
		if err != io.EOF {
			t.Fatal("data left in decoder:", m.Get())
		}
	}
}
