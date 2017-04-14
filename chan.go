package cmds

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

func NewChanResponsePair(req Request) (ResponseEmitter, Response) {
	ch := make(chan interface{})
	wait := make(chan struct{})

	r := &chanResponse{
		req:  req,
		ch:   ch,
		wait: wait,
	}

	re := &chanResponseEmitter{
		ch:     ch,
		length: &r.length,
		wait:   wait,
	}

	return re, r
}

type chanResponse struct {
	req Request

	err    *cmdsutil.Error
	length uint64

	// wait makes header requests block until the body is sent
	wait chan struct{}
	ch   <-chan interface{}
}

func (r *chanResponse) Request() Request {
	if r == nil {
		return nil
	}

	return r.req
}

func (r *chanResponse) Error() *cmdsutil.Error {
	<-r.wait

	if r == nil {
		return nil
	}

	return r.err
}

func (r *chanResponse) Length() uint64 {
	<-r.wait

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
		log.Debug("chResp.Next: got v=", v)
		if err, ok := v.(*cmdsutil.Error); ok {
			r.err = err
			return nil, ErrRcvdError
		} else {
			return v, nil
		}
	}

	return nil, io.EOF
}

type chanResponseEmitter struct {
	ch   chan<- interface{}
	wait chan struct{}

	length *uint64
	err    **cmdsutil.Error

	emitted bool
}

func (re *chanResponseEmitter) SetError(v interface{}, errType cmdsutil.ErrorType) {
	log.Debugf("re.SetError(%v, %v)", v, errType)
	re.Emit(&cmdsutil.Error{Message: fmt.Sprint(v), Code: errType})
}

func (re *chanResponseEmitter) SetLength(l uint64) {
	// don't change value after emitting
	if re.emitted {
		return
	}

	*re.length = l
}

func (re *chanResponseEmitter) Head() Head {
	<-re.wait

	return Head{
		Len: *re.length,
		Err: *re.err,
	}
}

func (re *chanResponseEmitter) Close() error {
	if re.ch == nil {
		return nil
	}

	log.Debug("closing chanRE ", re)
	close(re.ch)
	re.ch = nil

	return nil
}

func (re *chanResponseEmitter) Emit(v interface{}) error {
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
	if re.wait != nil {
		close(re.wait)
		re.wait = nil
	}

	if re.ch == nil {
		return fmt.Errorf("emitter closed")
	}

	re.ch <- v

	return nil
}
