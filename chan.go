package cmds

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
)

func NewChanResponsePair(req *Request) (ResponseEmitter, Response) {
	ch := make(chan interface{})
	wait := make(chan struct{})

	r := &chanResponse{
		req:    req,
		ch:     ch,
		wait:   wait,
		closed: make(chan struct{}),
	}

	re := (*chanResponseEmitter)(r)

	return re, r
}

type chanResponse struct {
	wl  sync.Mutex // lock for writing calls, i.e. Emit et al.
	rl  sync.Mutex // lock for reading calls, i.e. Next
	req *Request

	// wait makes header requests block until the body is sent
	wait chan struct{}
	// waitOnce makes sure we only close wait once
	waitOnce sync.Once

	// ch is used to send values from emitter to response
	ch chan interface{}

	emitted bool
	err     error
	length  uint64

	closeOnce sync.Once
	closed    chan struct{}
}

func (r *chanResponse) Request() *Request {
	if r == nil {
		return nil
	}

	return r.req
}

func (r *chanResponse) Error() *cmdkit.Error {
	<-r.wait

	if r.err == nil || r.err == io.EOF {
		return nil
	}

	if e, ok := r.err.(*cmdkit.Error); ok {
		return e
	}

	return &cmdkit.Error{Message: r.err.Error()}
}

func (r *chanResponse) Length() uint64 {
	<-r.wait

	return r.length
}

func (re *chanResponse) Head() Head {
	<-re.wait

	var err error
	if re.err != io.EOF {
		err = re.err
	}

	cmdErr, ok := err.(*cmdkit.Error)
	if !ok && err != nil {
		cmdErr = &cmdkit.Error{Message: err.Error()}
	}

	return Head{
		Len: re.length,
		Err: cmdErr,
	}
}

func (r *chanResponse) Next() (interface{}, error) {
	if r == nil {
		return nil, io.EOF
	}

	var ctx context.Context
	if rctx := r.req.Context; rctx != nil {
		ctx = rctx
	} else {
		ctx = context.Background()
	}

	// to avoid races by setting r.ch to nil
	r.rl.Lock()
	defer r.rl.Unlock()

	select {
	case <-r.closed:
		return nil, r.err
	case v, ok := <-r.ch:
		if !ok {
			r.ch = nil
			return nil, r.err
		}

		if err, ok := v.(*cmdkit.Error); ok {
			v = &err
		}

		switch val := v.(type) {
		case Single:
			return val.Value, nil
		default:
			return v, nil
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

}

func (r *chanResponse) RawNext() (interface{}, error) {
	if r == nil {
		return nil, io.EOF
	}

	var ctx context.Context
	if rctx := r.req.Context; rctx != nil {
		ctx = rctx
	} else {
		ctx = context.Background()
	}

	select {
	case v, ok := <-r.ch:
		if ok {
			return v, nil
		}

		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}

}

type chanResponseEmitter chanResponse

func (re *chanResponseEmitter) Emit(v interface{}) error {
	// Initially this library allowed commands to return errors by sending an
	// error value along a stream. We removed that in favour of CloseWithError,
	// so we want to make sure we catch situations where some code still uses the
	// old error emitting semantics.
	// Also errors may occur both as pointers and as plain values, so we need to
	// check both.
	if e, ok := v.(cmdkit.Error); ok {
		v = &e
	}
	if e, ok := v.(*cmdkit.Error); ok {
		log.Errorf("unexpected error value emitted: %s at\n%s", e.Error(), debug.Stack())
	}

	// channel emission iteration
	// TODO maybe remove this and use EmitChan instead of calling Emit directly?
	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}
	if ch, isChan := v.(<-chan interface{}); isChan {
		return EmitChan(re, ch)
	}

	// unblock Length(), Error() and Head()
	re.waitOnce.Do(func() {
		close(re.wait)
	})

	re.wl.Lock()
	defer re.wl.Unlock()

	if _, ok := v.(Single); ok {
		defer re.closeWithError(io.EOF)
	}

	ctx := re.req.Context

	select {
	case <-re.closed:
		return fmt.Errorf("emitter closed")
	case re.ch <- v:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (re *chanResponseEmitter) SetLength(l uint64) {
	re.wl.Lock()
	defer re.wl.Unlock()

	// don't change value after emitting
	if re.emitted {
		return
	}

	re.length = l
}

func (re *chanResponseEmitter) Close() error {
	return re.CloseWithError(nil)
}

func (re *chanResponseEmitter) CloseWithError(err error) error {
	re.wl.Lock()
	defer re.wl.Unlock()

	return re.closeWithError(err)
}

func (re *chanResponseEmitter) closeWithError(err error) error {
	if err == nil {
		err = io.EOF
	}

	if e, ok := err.(cmdkit.Error); ok {
		err = &e
	}

	re.closeOnce.Do(func() {
		re.err = err
		close(re.ch)
		close(re.closed)
	})

	// unblock Length(), Error() and Head()
	re.waitOnce.Do(func() {
		close(re.wait)
	})

	return nil
}
