package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

var EmittedErr = fmt.Errorf("received an error")

func NewWriterResponseEmitter(w io.WriteCloser, enc func(io.Writer) Encoder) ResponseEmitter {
	return &WriterResponseEmitter{
		w:   w,
		c:   w,
		enc: enc(w),
	}
}

func NewReaderResponse(r io.Reader, encType EncodingType, req Request) Response {
	return &readerResponse{
		req:     req,
		r:       r,
		encType: encType,
		dec:     Decoders[encType](r),
	}
}

type readerResponse struct {
	r       io.Reader
	encType EncodingType
	dec     Decoder

	req Request
	t   reflect.Type

	length uint64
	err    *cmdsutil.Error
}

func (r *readerResponse) Request() *Request {
	return &r.req
}

func (r *readerResponse) Error() *cmdsutil.Error {
	return r.err
}

func (r *readerResponse) Length() uint64 {
	return r.length
}

func (r *readerResponse) Next() (interface{}, error) {
	a := &Any{}
	a.Add(cmdsutil.Error{})
	a.Add(r.req.Command().Type)

	err := r.dec.Decode(a)
	if err != nil {
		return nil, err
	}

	v := a.Interface()
	if err, ok := v.(error); ok {
		return err, EmittedErr
	}

	return v, nil
}

type WriterResponseEmitter struct {
	w   io.Writer
	c   io.Closer
	enc Encoder

	length *uint64
	err    *cmdsutil.Error

	emitted bool
}

func (re *WriterResponseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	*re.err = cmdsutil.Error{Message: fmt.Sprint(err), Code: code}
}

func (re *WriterResponseEmitter) SetLength(length uint64) {
	if re.emitted {
		return
	}

	*re.length = length
}

func (re *WriterResponseEmitter) Close() error {
	return re.c.Close()
}

func (re *WriterResponseEmitter) Head() Head {
	return Head{
		Len: *re.length,
		Err: re.err,
	}
}

func (re *WriterResponseEmitter) SetEncoder(enc func(io.Writer) Encoder) {
	re.enc = enc(re.w)
}

func (re *WriterResponseEmitter) Emit(v interface{}) error {
	re.emitted = true

	return re.enc.Encode(v)
}

type Any struct {
	types []reflect.Type

	v interface{}
}

func (a *Any) UnmarshalJSON(data []byte) error {
	var (
		iv  interface{}
		err error
	)

	for _, t := range a.types {
		v := reflect.New(t)

		err = json.Unmarshal(data, v.Interface())
		if err == nil && v.Elem().Interface() != reflect.Zero(t).Interface() {
			a.v = v.Elem().Interface()
			return nil
		}
	}

	err = json.Unmarshal(data, &iv)
	a.v = iv

	return err
}

func (a *Any) Add(v interface{}) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Interface {
		t = t.Elem()
	}

	a.types = append(a.types, t)
}

func (a *Any) Interface() interface{} {
	return a.v
}
