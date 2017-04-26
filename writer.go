package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"

	"gx/ipfs/Qmf7G7FikwUsm48Jm4Yw4VBGNZuyRaAMzpWDJcW8V71uV2/go-ipfs-cmdkit"
)

func NewWriterResponseEmitter(w io.WriteCloser, req Request, enc func(Request) func(io.Writer) Encoder) *WriterResponseEmitter {
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

func NewReaderResponse(r io.Reader, encType EncodingType, req Request) Response {
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

	req Request

	length uint64
	err    *cmdsutil.Error

	emitted chan struct{}
	once    sync.Once
}

func (r *readerResponse) Request() Request {
	return r.req
}

func (r *readerResponse) Error() *cmdsutil.Error {
	<-r.emitted

	return r.err
}

func (r *readerResponse) Length() uint64 {
	<-r.emitted

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

	r.once.Do(func() { close(r.emitted) })

	v := a.Interface()
	if err, ok := v.(cmdsutil.Error); ok {
		r.err = &err
		return nil, ErrRcvdError
	}
	if err, ok := v.(*cmdsutil.Error); ok {
		r.err = err
		return nil, ErrRcvdError
	}

	return v, nil
}

type WriterResponseEmitter struct {
	// TODO maybe make those public?
	w   io.Writer
	c   io.Closer
	enc Encoder
	req Request

	length *uint64
	err    *cmdsutil.Error

	emitted bool
}

func (re *WriterResponseEmitter) SetEncoder(mkEnc func(io.Writer) Encoder) {
	re.enc = mkEnc(re.w)
}

func (re *WriterResponseEmitter) SetError(v interface{}, errType cmdsutil.ErrorType) error {
	return re.Emit(&cmdsutil.Error{Message: fmt.Sprint(v), Code: errType})
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
