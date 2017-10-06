package cmds

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs-cmdkit"
)

type TestOutput struct {
	Foo, Bar string
	Baz      int
}

func TestMarshalling(t *testing.T) {
	cmd := &Command{}
	opts, _ := cmd.GetOptions(nil)

	req, _ := NewRequest(nil, nil, nil, nil, nil, opts)

	buf := bytes.NewBuffer(nil)
	wc := writecloser{Writer: buf, Closer: nopCloser{}}
	re := NewWriterResponseEmitter(wc, req, Encoders[JSON])

	err := re.Emit(TestOutput{"beep", "boop", 1337})
	if err != nil {
		t.Error(err, "Should have passed")
	}

	req.SetOption(cmdkit.EncShort, JSON)

	output := buf.String()
	if removeWhitespace(output) != "{\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}" {
		t.Log("expected: {\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}")
		t.Log("got:", removeWhitespace(buf.String()))
		t.Error("Incorrect JSON output")
	}

	buf.Reset()

	re.SetError(fmt.Errorf("Oops!"), cmdkit.ErrClient)

	output = buf.String()
	if removeWhitespace(output) != `{"Message":"Oops!","Code":1,"Type":"error"}` {
		t.Log(`expected: {"Message":"Oops!","Code":1,"Type":"error"}`)
		t.Log("got:", removeWhitespace(buf.String()))
		t.Error("Incorrect JSON output")
	}
}

func removeWhitespace(input string) string {
	input = strings.Replace(input, " ", "", -1)
	input = strings.Replace(input, "\t", "", -1)
	input = strings.Replace(input, "\n", "", -1)
	return strings.Replace(input, "\r", "", -1)
}
