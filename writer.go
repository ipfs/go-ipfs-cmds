package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
)

func NewWriterResponseEmitter(w io.WriteCloser, req *Request, enc func(*Request) func(io.Writer) Encoder) *WriterResponseEmitter {
	re := &WriterResponseEmitter{
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
	emitted := make(chan struct{})

	return &readerResponse{
		req:     req,
		r:       r,
		encType: encType,
		dec:     Decoders[encType](r),
		emitted: emitted,
	}
}

type readerResponse struct {
	r       io.Reader
	encType EncodingType
	dec     Decoder

	req *Request

	length uint64
	err    *cmdkit.Error

	emitted chan struct{}
	once    sync.Once
}

func (r *readerResponse) Request() *Request {
	return r.req
}

func (r *readerResponse) Error() *cmdkit.Error {
	<-r.emitted

	return r.err
}

func (r *readerResponse) Length() uint64 {
	<-r.emitted

	return r.length
}

func (r *readerResponse) RawNext() (interface{}, error) {
	a := &Any{}
	a.Add(cmdkit.Error{})
	a.Add(r.req.Command.Type)

	err := r.dec.Decode(a)
	if err != nil {
		return nil, err
	}

	r.once.Do(func() { close(r.emitted) })

	v := a.Interface()
	return v, nil
}

func (r *readerResponse) Next() (interface{}, error) {
	a := &Any{}
	a.Add(cmdkit.Error{})
	a.Add(r.req.Command.Type)

	err := r.dec.Decode(a)
	if err != nil {
		return nil, err
	}

	r.once.Do(func() { close(r.emitted) })

	v := a.Interface()
	if err, ok := v.(cmdkit.Error); ok {
		v = &err
	}

	switch val := v.(type) {
	case *cmdkit.Error:
		r.err = val
		return nil, ErrRcvdError
	case Single:
		return val.Value, nil
	default:
		return v, nil
	}
}

type WriterResponseEmitter struct {
	// TODO maybe make those public?
	w   io.Writer
	c   io.Closer
	enc Encoder
	req *Request

	length *uint64
	err    *cmdkit.Error

	emitted bool
}

func (re *WriterResponseEmitter) SetEncoder(mkEnc func(io.Writer) Encoder) {
	re.enc = mkEnc(re.w)
}

func (re *WriterResponseEmitter) SetError(v interface{}, errType cmdkit.ErrorType) {
	err := re.Emit(&cmdkit.Error{Message: fmt.Sprint(v), Code: errType})
	if err != nil {
		panic(err)
	}
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

func (re *WriterResponseEmitter) Emit(v interface{}) error {
	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}

	if ch, isChan := v.(<-chan interface{}); isChan {
		for v = range ch {
			err := re.Emit(v)
			if err != nil {
				return err
			}
		}
		return nil
	}

	re.emitted = true

	if _, ok := v.(Single); ok {
		defer re.Close()
	}

	return re.enc.Encode(v)
}

type Any struct {
	types map[reflect.Type]bool
	order []reflect.Type

	v interface{}
}

func (a *Any) UnmarshalJSON(data []byte) error {
	var (
		iv  interface{}
		err error
	)

	for _, t := range a.order {
		v := reflect.New(t).Elem().Addr()

		isNil := func(v reflect.Value) (yup, ok bool) {
			ok = true
			defer func() {
				r := recover()
				if r != nil {
					ok = false
				}
			}()
			yup = v.IsNil()
			return
		}

		isZero := func(v reflect.Value, t reflect.Type) (yup, ok bool) {
			ok = true
			defer func() {
				r := recover()
				if r != nil {
					ok = false
				}
			}()
			yup = v.Elem().Interface() == reflect.Zero(t).Interface()
			return
		}

		err = json.Unmarshal(data, v.Interface())

		vIsNil, isNilOk := isNil(v)
		vIsZero, isZeroOk := isZero(v, t)

		nilish := (isNilOk && vIsNil) || (isZeroOk && vIsZero)
		if err == nil && !nilish {
			a.v = v.Interface()
			return nil
		}
	}

	err = json.Unmarshal(data, &iv)
	a.v = iv

	return err
}

func (a *Any) Add(v interface{}) {
	if v == nil {
		return
	}
	if a.types == nil {
		a.types = map[reflect.Type]bool{}
	}
	t := reflect.TypeOf(v)
	isPtr := t.Kind() == reflect.Ptr
	if isPtr || t.Kind() == reflect.Interface {
		t = t.Elem()
	}

	a.types[t] = isPtr
	a.order = append(a.order, t)
}

func (a *Any) Interface() interface{} {
	return a.v
}
