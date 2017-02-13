package cmds

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
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

	err    *cmdsutil.Error
	length uint64

	ch <-chan interface{}
}

func (r *chanResponse) Request() *Request {
	if r == nil {
		return nil
	}

	return r.req
}

func (r *chanResponse) Error() *cmdsutil.Error {
	if r == nil {
		return nil
	}

	return r.err
}

func (r *chanResponse) Length() uint64 {
	if r == nil {
		return 0
	}

	return r.length
}

func (r *chanResponse) Next() (interface{}, error) {
	if r == nil {
		return nil, io.EOF
	}

	v, ok := <-r.ch
	if ok {
		return v, nil
	}

	return nil, io.EOF
}

type chanResponseEmitter struct {
	ch chan<- interface{}

	length *uint64
	err    **cmdsutil.Error

	emitted bool
}

func (re *chanResponseEmitter) SetError(err interface{}, t cmdsutil.ErrorType) {
	// don't change value after emitting
	/*
		if re.emitted {
			return
		}
	*/

	*re.err = &cmdsutil.Error{Message: fmt.Sprint(err), Code: t}
}

func (re *chanResponseEmitter) SetLength(l uint64) {
	// don't change value after emitting
	if re.emitted {
		return
	}

	*re.length = l
}

func (re *chanResponseEmitter) Head() Head {
	return Head{
		Len: *re.length,
		Err: *re.err,
	}
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
