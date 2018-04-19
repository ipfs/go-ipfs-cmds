package cmds

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
)

func EmitChan(re ResponseEmitter, ch chan interface{}) error {
	for v := range ch {
		err := re.Emit(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewChanResponsePair(req *Request) (ResponseEmitter, Response) {
	ch := make(chan interface{})
	wait := make(chan struct{})

	r := &chanResponse{
		req:  req,
		ch:   ch,
		wait: wait,
	}

	re := (*chanResponseEmitter)(r)

	return re, r
}

type chanResponse struct {
	l   sync.Mutex
	req *Request

	// wait makes header requests block until the body is sent
	wait chan struct{}
	// waitOnce makes sure we only close wait once
	waitOnce sync.Once

	// ch is used to send values from emitter to response
	ch chan interface{}

	emitted bool
	err     *cmdkit.Error
	length  uint64
}

func (r *chanResponse) Request() *Request {
	if r == nil {
		return nil
	}

	return r.req
}

func (r *chanResponse) Error() *cmdkit.Error {
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

func (re *chanResponse) Head() Head {
	<-re.wait

	return Head{
		Len: re.length,
		Err: re.err,
	}
}

func (r *chanResponse) Next() (interface{}, error) {
	if r == nil {
		if r.err != nil {
			return nil, r.err
		}

		return nil, io.EOF
	}

	var ctx context.Context
	if rctx := r.req.Context; rctx != nil {
		ctx = rctx
	} else {
		ctx = context.Background()
	}

	err := func() error {
		if r.ch == nil {
			return io.EOF
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	select {
	case v, ok := <-r.ch:
		if !ok {
			if r.err != nil {
				return nil, r.err
			}

			return nil, io.EOF
		}

		if err, ok := v.(*cmdkit.Error); ok {
			v = &err
		}

		switch val := v.(type) {
		case *cmdkit.Error:
			// TODO keks remove logging
			log.Error("unexpected error value:", val)
			return val, nil
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
	if e, ok := v.(*cmdkit.Error); ok {
		log.Errorf("unexpected error value emitted: %s at\n%s", e.Error(), debug.Stack())
	}

	// channel emission iteration
	// TODO remove this
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

	// unblock Length(), Error() and Head()
	re.waitOnce.Do(func() {
		close(re.wait)
	})

	re.l.Lock()
	defer re.l.Unlock()

	if re.ch == nil {
		return fmt.Errorf("emitter closed")
	}

	if _, ok := v.(Single); ok {
		defer re.close()
	}

	ctx := re.req.Context

	fmt.Println("emitting", re.ch == nil)

	select {
	case re.ch <- v:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (re *chanResponseEmitter) CloseWithError(err error) error {
	re.l.Lock()
	defer re.l.Unlock()

	e, ok := err.(*cmdkit.Error)
	if !ok {
		e = &cmdkit.Error{Message: err.Error()}
	}

	re.err = e
	return re.close()
}

func (re *chanResponseEmitter) SetLength(l uint64) {
	re.l.Lock()
	defer re.l.Unlock()

	// don't change value after emitting
	if re.emitted {
		return
	}

	re.length = l
}

func (re *chanResponseEmitter) close() error {
	fmt.Printf("close called %p\n", re)
	if re.ch == nil {
		return nil
	}

	close(re.ch)
	re.ch = nil

	return nil
}

func (re *chanResponseEmitter) Close() error {
	re.l.Lock()
	defer re.l.Unlock()

	return re.close()
}
