package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

var EmittedErr = fmt.Errorf("received an error")

func NewWriterResponseEmitter(w io.WriteCloser, res Response, enc func(io.Writer) Encoder) *WriterResponseEmitter {
	return &WriterResponseEmitter{
		w:   w,
		c:   w,
		enc: enc(w),
		req: res.Request(),
	}
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
	t   reflect.Type

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
	if err, ok := v.(error); ok {
		return err, EmittedErr
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
	tees    []ResponseEmitter
}

func (re *WriterResponseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	*re.err = cmdsutil.Error{Message: fmt.Sprint(err), Code: code}

	for _, re_ := range re.tees {
		re_.SetError(err, code)
	}
}

func (re *WriterResponseEmitter) SetLength(length uint64) {
	if re.emitted {
		return
	}

	*re.length = length

	for _, re_ := range re.tees {
		re_.SetLength(length)
	}
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
	re.emitted = true

	err := re.enc.Encode(v)
	if err != nil {
		return err
	}

	for _, re_ := range re.tees {
		err = re_.Emit(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (re *WriterResponseEmitter) Tee(re_ ResponseEmitter) {
	re.tees = append(re.tees, re_)

	// TODO first check whether length and error have been set
	re_.SetLength(*re.length)
	re_.SetError(re.err.Message, re.err.Code)
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
