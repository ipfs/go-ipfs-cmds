package cmds

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
)

// Encoder encodes values onto e.g. an io.Writer. Examples are json.Encoder and xml.Encoder.
type Encoder interface {
	Encode(value interface{}) error
}

// Decoder decodes values into value (which should be a pointer).
type Decoder interface {
	Decode(value interface{}) error
}

// EncodingType defines a supported encoding
type EncodingType string

// Supported EncodingType constants.
const (
	Undefined = ""

	JSON        = "json"
	XML         = "xml"
	Protobuf    = "protobuf"
	Text        = "text"
	TextNewline = "textnl"
	CLI         = "cli"

	// TODO: support more encoding types
)

var Decoders = map[EncodingType]func(w io.Reader) Decoder{
	XML: func(r io.Reader) Decoder {
		return xml.NewDecoder(r)
	},
	JSON: func(r io.Reader) Decoder {
		return json.NewDecoder(r)
	},
}

type EncoderFunc func(req Request) func(w io.Writer) Encoder
type EncoderMap map[EncodingType]EncoderFunc

var Encoders = EncoderMap{
	XML: func(req Request) func(io.Writer) Encoder {
		return func(w io.Writer) Encoder { return xml.NewEncoder(w) }
	},
	JSON: func(req Request) func(io.Writer) Encoder {
		return func(w io.Writer) Encoder { return json.NewEncoder(w) }
	},
	Text: func(req Request) func(io.Writer) Encoder {
		return func(w io.Writer) Encoder { return TextEncoder{w: w} }
	},
	TextNewline: func(req Request) func(io.Writer) Encoder {
		return func(w io.Writer) Encoder { return TextEncoder{w: w, suffix: "\n"} }
	},
}

func MakeEncoder(f func(Request, io.Writer, interface{}) error) func(Request) func(io.Writer) Encoder {
	return func(req Request) func(io.Writer) Encoder {
		return func(w io.Writer) Encoder { return &genericEncoder{f: f, w: w, req: req} }
	}
}

type genericEncoder struct {
	f   func(Request, io.Writer, interface{}) error
	w   io.Writer
	req Request
}

func (e *genericEncoder) Encode(v interface{}) error {
	return e.f(e.req, e.w, v)
}

type TextEncoder struct {
	w      io.Writer
	suffix string
}

func (e TextEncoder) Encode(v interface{}) error {
	_, err := fmt.Fprintf(e.w, "%s%s", v, e.suffix)
	return err
}
