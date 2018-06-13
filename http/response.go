package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"reflect"
)

var (
	MIMEEncodings = map[string]cmds.EncodingType{
		"application/json": cmds.JSON,
		"application/xml":  cmds.XML,
		"text/plain":       cmds.Text,
	}
)

type Response struct {
	length uint64
	err    *cmdkit.Error

	res *http.Response
	req *cmds.Request

	rr  *responseReader
	dec cmds.Decoder

	initErr *cmdkit.Error
}

func (res *Response) Request() *cmds.Request {
	return res.req
}

func (res *Response) Error() *cmdkit.Error {
	if res.err == io.EOF {
		return nil
	}

	return res.err
}

func (res *Response) Length() uint64 {
	return res.length
}

func (res *Response) RawNext() (interface{}, error) {
	if res.initErr != nil {
		return nil, res.initErr
	}

	if res.err != nil {
		return nil, res.err
	}

	// nil decoder means stream not chunks
	// but only do that once
	if res.dec == nil {
		if res.rr == nil {
			return nil, io.EOF
		}
		rr := res.rr
		res.rr = nil
		return rr, nil
	}

	var value interface{}
	if valueType := reflect.TypeOf(res.req.Command.Type); valueType != nil {
		if valueType.Kind() == reflect.Ptr {
			valueType = valueType.Elem()
		}
		value = reflect.New(valueType).Interface()
	}
	m := &cmds.MaybeError{Value: value}
	err := res.dec.Decode(m)
	if err != nil {
		if res.err != nil && err.Error() == res.err.Error() {
			// last error was sent as value, now we get the same error from the headers. ignore and EOF!
			err = io.EOF
		} else if err == io.EOF {
			// just return EOF errors
			return nil, err
		} else {
			// wrap all other errors
			res.err = &cmdkit.Error{Message: err.Error()}
		}
	}

	v, err := m.Get()
	if err != nil {
		if e, ok := err.(*cmdkit.Error); ok {
			res.err = e
		} else {
			res.err = &cmdkit.Error{Message: err.Error()}
		}
	}

	return v, err
}

func (res *Response) Next() (interface{}, error) {
	v, err := res.RawNext()
	if err != nil {
		return nil, err
	}

	if err, ok := v.(cmdkit.Error); ok {
		v = &err
	}

	switch val := v.(type) {
	case *cmdkit.Error:
		res.err = val
		return nil, val
	default:
		return v, nil
	}
}

// responseReader reads from the response body, and checks for an error
// in the http trailer upon EOF, this error if present is returned instead
// of the EOF.
type responseReader struct {
	resp *http.Response
}

func (r *responseReader) Read(b []byte) (int, error) {
	if r == nil || r.resp == nil {
		return 0, io.EOF
	}

	n, err := r.resp.Body.Read(b)

	// reading on a closed response body is as good as an io.EOF here
	if err != nil && strings.Contains(err.Error(), "read on closed response body") {
		err = io.EOF
	}
	if err == io.EOF {
		_ = r.resp.Body.Close()
		trailerErr := r.checkError()
		if trailerErr != nil {
			return n, trailerErr
		}
	}
	return n, err
}

func (r *responseReader) checkError() error {
	if e := r.resp.Trailer.Get(StreamErrHeader); e != "" {
		return errors.New(e)
	}
	return nil
}

func (r *responseReader) Close() error {
	return r.resp.Body.Close()
}
