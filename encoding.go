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
	JSON     = "json"
	XML      = "xml"
	Protobuf = "protobuf"
	Text     = "text"
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

var Encoders = map[EncodingType]func(w io.Writer) Encoder{
	XML: func(w io.Writer) Encoder {
		return xml.NewEncoder(w)
	},
	JSON: func(w io.Writer) Encoder {
		return json.NewEncoder(w)
	},
	Text: func(w io.Writer) Encoder {
		return textEncoder{w}
	},
}

type textEncoder struct {
	w io.Writer
}

func (e textEncoder) Encode(v interface{}) error {
	_, err := fmt.Fprint(e.w, v)
	return err
}
