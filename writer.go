package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds/debug"
)

func NewWriterResponseEmitter(w io.WriteCloser, req *Request, enc func(*Request) func(io.Writer) Encoder) ResponseEmitter {
	re := &writerResponseEmitter{
		w:   w,
		c:   w,
		req: req,
	}

	if enc != nil {
		re.enc = enc(req)(w)
	}

	return re
}

func NewReaderResponse(r io.Reader, encType EncodingType, req *Request) Response {
	return &readerResponse{
		req:     req,
		r:       r,
		encType: encType,
		dec:     Decoders[encType](r),
		emitted: make(chan struct{}),
	}
}

type readerResponse struct {
	r       io.Reader
	encType EncodingType
	dec     Decoder

	req *Request

	length uint64
	err    error

	emitted chan struct{}
	once    sync.Once
}

func (r *readerResponse) Request() *Request {
	return r.req
}

func (r *readerResponse) Error() *cmdkit.Error {
	<-r.emitted

	if err, ok := r.err.(*cmdkit.Error); ok {
		return err
	}

	return &cmdkit.Error{Message: r.err.Error()}
}

func (r *readerResponse) Length() uint64 {
	<-r.emitted

	return r.length
}

func (r *readerResponse) Next() (interface{}, error) {
	m := &MaybeError{Value: r.req.Command.Type}
	err := r.dec.Decode(m)
	if err != nil {
		return nil, err
	}

	r.once.Do(func() { close(r.emitted) })

	v, err := m.Get()

	// because working with pointers to arrays is annoying
	if t := reflect.TypeOf(v); t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Slice {
		v = reflect.ValueOf(v).Elem().Interface()
	}
	return v, err
}

type writerResponseEmitter struct {
	// TODO maybe make those public?
	w   io.Writer
	c   io.Closer
	enc Encoder
	req *Request

	length *uint64
	err    *cmdkit.Error

	emitted bool
}

func (re *writerResponseEmitter) SetEncoder(mkEnc func(io.Writer) Encoder) {
	re.enc = mkEnc(re.w)
}

func (re *writerResponseEmitter) CloseWithError(err error) error {
	cwe, ok := re.c.(interface{ CloseWithError(error) error })
	if ok {
		return cwe.CloseWithError(err)
	}

	if err == nil || err == io.EOF {
		return re.Close()
	}

	return errors.New("provided closer does not support CloseWithError")
}

func (re *writerResponseEmitter) SetError(v interface{}, errType cmdkit.ErrorType) {
	err := re.Emit(&cmdkit.Error{Message: fmt.Sprint(v), Code: errType})
	if err != nil {
		panic(err)
	}
}

func (re *writerResponseEmitter) SetLength(length uint64) {
	if re.emitted {
		return
	}

	*re.length = length
}

func (re *writerResponseEmitter) Close() error {
	return re.c.Close()
}

func (re *writerResponseEmitter) Emit(v interface{}) error {
	// channel emission iteration
	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}
	if ch, isChan := v.(<-chan interface{}); isChan {
		return EmitChan(re, ch)
	}

	// Initially this library allowed commands to return errors by sending an
	// error value along a stream. We removed that in favour of CloseWithError,
	// so we want to make sure we catch situations where some code still uses the
	// old error emitting semantics.
	// Also errors may occur both as pointers and as plain values, so we need to
	// check both.
	debug.AssertNotError(v)

	if _, ok := v.(Single); ok {
		defer re.Close()
	}

	re.emitted = true

	return re.enc.Encode(v)
}

type MaybeError struct {
	Value interface{} // needs to be a pointer
	Error *cmdkit.Error

	isError bool
}

func (m *MaybeError) Get() (interface{}, error) {
	if m.isError {
		return nil, m.Error
	}
	return m.Value, nil
}

func (m *MaybeError) UnmarshalJSON(data []byte) error {
	var e cmdkit.Error
	err := json.Unmarshal(data, &e)
	if err == nil {
		m.isError = true
		m.Error = &e
		return nil
	}

	if m.Value != nil {
		// make sure we are working with a pointer here
		v := reflect.ValueOf(m.Value)
		if v.Kind() != reflect.Ptr {
			m.Value = reflect.New(v.Type()).Interface()
		}

		err = json.Unmarshal(data, m.Value)
	} else {
		// let the json decoder decode into whatever it finds appropriate
		err = json.Unmarshal(data, &m.Value)
	}

	return err
}
