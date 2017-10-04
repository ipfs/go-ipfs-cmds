package cmdkit

import (
	"encoding/json"
	"testing"
)

func TestMarshal(t *testing.T) {
	e := Error{
		Message: "error msg",
	}

	buf, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string]interface{})

	err = json.Unmarshal(buf, &m)
	if err != nil {
		t.Fatal(err)
	}

	if len(m) != 3 {
		t.Fatal("expected three map elements, got ", len(m))
	}

	if m["Message"].(string) != "error msg" {
		t.Fatal(`expected m["Message"] == "error msg", got "`, m["Message"], `"`)
	}

	if m["Code"].(float64) != 0 {
		t.Fatal(`expected m["Code"] == 0, got "`, m["Code"], `"`)
	}

	if m["Type"].(string) != "error" {
		t.Fatal(`expected m["Type"] == "error", got "`, m["Type"], `"`)
	}

	e = Error{}
	t.Logf("%s\n", buf)

	err = json.Unmarshal(buf, &e)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v\n", e)

	if e.Message != "error msg" {
		t.Fatal(`expected e.Message == "error msg", got "` + e.Message + `"`)
	}

	if e.Code != 0 {
		t.Fatal(`expected e.Code == 0, got "`, e.Code, `"`)
	}
}
