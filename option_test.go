package cmdkit

import (
	"strings"
	"testing"
)

func TestOptionValueExtractBoolNotFound(t *testing.T) {
	t.Log("ensure that no error is returned when value is not found")
	optval := &OptionValue{ValueFound: false}
	_, _, err := optval.Bool()
	if err != nil {
		t.Fatal("Found was false. Err should have been nil")
	}
}

func TestOptionValueExtractWrongType(t *testing.T) {

	t.Log("ensure that error is returned when value if of wrong type")

	optval := &OptionValue{Value: "wrong type: a string", ValueFound: true}
	_, _, err := optval.Bool()
	if err == nil {
		t.Fatal("No error returned. Failure.")
	}

	optval = &OptionValue{Value: "wrong type: a string", ValueFound: true}
	_, _, err = optval.Int()
	if err == nil {
		t.Fatal("No error returned. Failure.")
	}
}

func TestLackOfDescriptionOfOptionDoesNotPanic(t *testing.T) {
	opt := BoolOption("a", "")
	opt.Description()
}

func TestDotIsAddedInDescripton(t *testing.T) {
	opt := BoolOption("a", "desc without dot")
	dest := opt.Description()
	if !strings.HasSuffix(dest, ".") {
		t.Fatal("dot should have been added at the end of description")
	}
}
