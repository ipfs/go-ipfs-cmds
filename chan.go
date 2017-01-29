package cmds

import (
	"fmt"
	"io"
)

func NewChanResponsePair(req *Request) (ResponseEmitter, Response) {
	ch := make(chan interface{})

	r := &chanResponse{
		req: req,
		ch:  ch,
	}

	re := &chanResponseEmitter{
		ch:     ch,
		length: &r.length,
		err:    &r.err,
	}

	return re, r
}

type chanResponse struct {
	req *Request

	err    *Error
	length uint64

	ch <-chan interface{}
}

func (r *chanResponse) Request() *Request {
	return r.req
}

func (r *chanResponse) Error() *Error {
	return r.err
}

func (r *chanResponse) Length() uint64 {
	return r.length
}

func (r *chanResponse) Next() (interface{}, error) {
	v, ok := <-r.ch
	if ok {
		return v, nil
	}

	return nil, io.EOF
}

type chanResponseEmitter struct {
	ch chan<- interface{}

	length *uint64
	err    **Error

	emitted bool
}

func (re *chanResponseEmitter) SetError(err interface{}, t ErrorType) {
	// don't change value after emitting
	if re.emitted {
		return
	}

	*re.err = &Error{Message: fmt.Sprint(err), Code: t}
}

func (re *chanResponseEmitter) SetLength(l uint64) {
	// don't change value after emitting
	if re.emitted {
		return
	}

	*re.length = l
}

func (re *chanResponseEmitter) Close() error {
	close(re.ch)
	re.ch = nil

	return nil
}

func (re *chanResponseEmitter) Emit(v interface{}) error {
	re.emitted = true

	if re.ch == nil {
		return fmt.Errorf("emitter closed")
	}

	re.ch <- v

	return nil
}
