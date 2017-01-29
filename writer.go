package cmds

import (
	"fmt"
	"io"
	"reflect"
)

func NewPipeResponsePair(encType EncodingType, req Request) (ResponseEmitter, Response) {
	r, w := io.Pipe()

	res := NewReaderResponse(r, encType, req)
	re := NewWriterResponseEmitter(w, encType)

	return re, res
}

func NewWriterResponseEmitter(w io.WriteCloser, encType EncodingType) ResponseEmitter {
	return &writerResponseEmitter{
		w:       w,
		encType: encType,
		enc:     Encoders[encType](w),
	}
}

func NewReaderResponse(r io.Reader, encType EncodingType, req Request) Response {
	return &readerResponse{
		req:     req,
		r:       r,
		encType: encType,
		dec:     Decoders[encType](r),
		t:       reflect.TypeOf(req.Command().Type),
	}
}

type readerResponse struct {
	r       io.Reader
	encType EncodingType
	dec     Decoder

	req Request
	t   reflect.Type

	length uint64
	err    *Error
}

func (r *readerResponse) Request() *Request {
	return &r.req
}

func (r *readerResponse) Error() *Error {
	return r.err
}

func (r *readerResponse) Length() uint64 {
	return r.length
}

func (r *readerResponse) Next() (interface{}, error) {
	v := reflect.New(r.t).Interface()
	err := r.dec.Decode(&v)

	return v, err
}

type writerResponseEmitter struct {
	w       io.WriteCloser
	encType EncodingType
	enc     Encoder

	length *uint64
	err    *Error

	emitted bool
}

func (re *writerResponseEmitter) SetError(err interface{}, code ErrorType) {
	if re.emitted {
		return
	}

	*re.err = Error{Message: fmt.Sprint(err), Code: code}
}

func (re *writerResponseEmitter) SetLength(length uint64) {
	if re.emitted {
		return
	}

	*re.length = length
}

func (re *writerResponseEmitter) Close() error {
	return re.w.Close()
}

func (re *writerResponseEmitter) Emit(v interface{}) error {
	re.emitted = true

	return re.enc.Encode(v)
}
