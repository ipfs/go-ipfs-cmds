package cmds

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/config"
)

// responseWrapper wraps Response and implements olcdms.Response.
// It embeds a Response so some methods are taken from that.
type responseWrapper struct {
	Response

	out interface{}
}

// Request returns a (faked) oldcmds.Request
func (rw *responseWrapper) Request() oldcmds.Request {
	return &requestWrapper{rw.Response.Request()}
}

// Output returns either a <-chan interface{} on which you can receive the
// emitted values, or an emitted io.Reader
func (rw *responseWrapper) Output() interface{} {
	//if not called before
	if rw.out == nil {
		// get first emitted value
		x, err := rw.Next()
		if err != nil {
			return nil
		}
		if e, ok := x.(*cmdkit.Error); ok {
			ch := make(chan interface{})
			log.Error(e)
			close(ch)
			return (<-chan interface{})(ch)
		}

		switch v := x.(type) {
		case io.Reader:
			// if it's a reader, set it
			rw.out = v
		default:
			// if it is something else, create a channel and copy values from next in there
			ch := make(chan interface{})
			rw.out = (<-chan interface{})(ch)

			go func() {
				defer close(ch)
				ch <- v

				for {
					v, err := rw.Next()

					if err == io.EOF || err == context.Canceled {
						return
					} else if err != nil {
						log.Error(err)
						return
					}

					ch <- v
				}
			}()
		}
	}

	// if we have it already, return existing value
	return rw.out
}

// SetError is an empty stub
func (rw *responseWrapper) SetError(error, cmdkit.ErrorType) {}

// SetOutput is an empty stub
func (rw *responseWrapper) SetOutput(interface{}) {}

// SetLength is an empty stub
func (rw *responseWrapper) SetLength(uint64) {}

// SetCloser is an empty stub
func (rw *responseWrapper) SetCloser(io.Closer) {}

// Close is an empty stub
func (rw *responseWrapper) Close() error { return nil }

// Marshal is an empty stub
func (rw *responseWrapper) Marshal() (io.Reader, error) { return nil, nil }

// Reader is an empty stub
func (rw *responseWrapper) Reader() (io.Reader, error) { return nil, nil }

// Stdout returns os.Stdout
func (rw *responseWrapper) Stdout() io.Writer { return os.Stdout }

// Stderr returns os.Stderr
func (rw *responseWrapper) Stderr() io.Writer { return os.Stderr }

// WrapOldRequest returns a faked Request from an oldcmds.Request.
func WrapOldRequest(r oldcmds.Request) Request {
	return &oldRequestWrapper{r}
}

// requestWrapper implements a oldcmds.Request from an Request
type requestWrapper struct {
	Request
}

// InvocContext retuns the invocation context of the oldcmds.Request.
// It is faked using OldContext().
func (r *requestWrapper) InvocContext() *oldcmds.Context {
	ctx := OldContext(*r.Request.InvocContext())
	return &ctx
}

// SetInvocContext sets the invocation context. First the context is converted
// to a Context using NewContext().
func (r *requestWrapper) SetInvocContext(ctx oldcmds.Context) {
	r.Request.SetInvocContext(NewContext(ctx))
}

// Command is an empty stub.
func (r *requestWrapper) Command() *oldcmds.Command { return nil }

// oldRequestWrapper implements a Request from an oldcmds.Request
type oldRequestWrapper struct {
	oldcmds.Request
}

// InvocContext retuns the invocation context of the oldcmds.Request.
// It is faked using NewContext().
func (r *oldRequestWrapper) InvocContext() *Context {
	ctx := NewContext(*r.Request.InvocContext())
	return &ctx
}

func (r *oldRequestWrapper) SetInvocContext(ctx Context) {
	r.Request.SetInvocContext(OldContext(ctx))
}

// Command is an empty stub
func (r *oldRequestWrapper) Command() *Command { return nil }

///

// fakeResponse implements oldcmds.Response and takes a ResponseEmitter
type fakeResponse struct {
	req  oldcmds.Request
	re   ResponseEmitter
	out  interface{}
	wait chan struct{}
	once sync.Once
}

// Send emits the value(s) stored in r.out on the ResponseEmitter
func (r *fakeResponse) Send(errCh chan<- error) {
	defer close(errCh)

	out := r.Output()
	if out == nil {
		return
	}

	if ch, ok := out.(chan interface{}); ok {
		out = (<-chan interface{})(ch)
	}

	err := r.re.Emit(out)
	errCh <- err
	return
}

// Request returns the oldcmds.Request that belongs to this Response
func (r *fakeResponse) Request() oldcmds.Request {
	return r.req
}

// SetError forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) SetError(err error, code cmdkit.ErrorType) {
	defer r.once.Do(func() { close(r.wait) })
	r.re.SetError(err, code)
}

// Error is an empty stub
func (r *fakeResponse) Error() *cmdkit.Error {
	return nil
}

// SetOutput sets the output variable to the passed value
func (r *fakeResponse) SetOutput(v interface{}) {
	t := reflect.TypeOf(v)
	_, isReader := v.(io.Reader)

	if t != nil && t.Kind() != reflect.Chan && !isReader {
		v = Single{v}
	}

	r.out = v
	r.once.Do(func() { close(r.wait) })
}

// Output returns the output variable
func (r *fakeResponse) Output() interface{} {
	<-r.wait
	return r.out
}

// SetLength forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) SetLength(l uint64) {
	r.re.SetLength(l)
}

// Length is an empty stub
func (r *fakeResponse) Length() uint64 {
	return 0
}

// Close forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) Close() error {
	return r.re.Close()
}

// SetCloser is an empty stub
func (r *fakeResponse) SetCloser(io.Closer) {}

// Reader is an empty stub
func (r *fakeResponse) Reader() (io.Reader, error) {
	return nil, nil
}

// Marshal is an empty stub
func (r *fakeResponse) Marshal() (io.Reader, error) {
	return nil, nil
}

// Stdout returns os.Stdout
func (r *fakeResponse) Stdout() io.Writer {
	return os.Stdout
}

// Stderr returns os.Stderr
func (r *fakeResponse) Stderr() io.Writer {
	return os.Stderr
}

///

// MarshalerEncoder implements Encoder from a Marshaler
type MarshalerEncoder struct {
	m   oldcmds.Marshaler
	w   io.Writer
	req Request
}

// NewMarshalerEncoder returns a new MarshalerEncoder
func NewMarshalerEncoder(req Request, m oldcmds.Marshaler, w io.Writer) *MarshalerEncoder {
	me := &MarshalerEncoder{
		m:   m,
		w:   w,
		req: req,
	}

	return me
}

// Encode encodes v onto the io.Writer w using Marshaler m, with both m and w passed in NewMarshalerEncoder
func (me *MarshalerEncoder) Encode(v interface{}) error {
	re, res := NewChanResponsePair(me.req)
	go re.Emit(v)

	r, err := me.m(&responseWrapper{Response: res})
	if err != nil {
		return err
	}
	if r == nil {
		// behave like empty reader
		return nil
	}

	_, err = io.Copy(me.w, r)
	return err
}

// wrappedResponseEmitter implements a ResponseEmitter by forwarding everything to an oldcmds.Response
type wrappedResponseEmitter struct {
	r oldcmds.Response
}

// SetLength forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) SetLength(l uint64) {
	re.r.SetLength(l)
}

// SetError forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) SetError(err interface{}, code cmdkit.ErrorType) {
	re.r.SetError(fmt.Errorf("%v", err), code)
}

// Close forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) Close() error {
	return re.r.Close()
}

// Emit sends the value to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) Emit(v interface{}) error {
	if re.r.Output() == nil {
		switch c := v.(type) {
		case io.Reader:
			re.r.SetOutput(c)
			return nil
		case chan interface{}:
			re.r.SetOutput(c)
			return nil
		case <-chan interface{}:
			re.r.SetOutput(c)
			return nil
		default:
			re.r.SetOutput(make(chan interface{}))
		}
	}

	go func() {
		re.r.Output().(chan interface{}) <- v
	}()

	return nil
}

// OldCommand returns an oldcmds.Command from a Command.
func OldCommand(cmd *Command) *oldcmds.Command {
	oldcmd := &oldcmds.Command{
		Options:   cmd.Options,
		Arguments: cmd.Arguments,
		Helptext:  cmd.Helptext,
		External:  cmd.External,
		Type:      cmd.Type,

		Subcommands: func() map[string]*oldcmds.Command {
			cs := make(map[string]*oldcmds.Command)

			for k, v := range cmd.OldSubcommands {
				cs[k] = v
			}

			for k, v := range cmd.Subcommands {
				cs[k] = OldCommand(v)
			}

			return cs
		}(),
	}

	if cmd.Run != nil {
		oldcmd.Run = func(oldReq oldcmds.Request, res oldcmds.Response) {
			req := &oldRequestWrapper{oldReq}
			re := &wrappedResponseEmitter{res}

			cmd.Run(req, re)
		}
	}
	if cmd.PreRun != nil {
		oldcmd.PreRun = func(oldReq oldcmds.Request) error {
			req := &oldRequestWrapper{oldReq}
			return cmd.PreRun(req)
		}
	}

	return oldcmd
}

// NewCommand returns a Command from an oldcmds.Command
func NewCommand(oldcmd *oldcmds.Command) *Command {
	if oldcmd == nil {
		return nil
	}
	var cmd *Command

	cmd = &Command{
		Options:   oldcmd.Options,
		Arguments: oldcmd.Arguments,
		Helptext:  oldcmd.Helptext,
		External:  oldcmd.External,
		Type:      oldcmd.Type,

		OldSubcommands: oldcmd.Subcommands,
	}

	if oldcmd.Run != nil {
		cmd.Run = func(req Request, re ResponseEmitter) {
			oldReq := &requestWrapper{req}
			res := &fakeResponse{req: oldReq, re: re, wait: make(chan struct{})}

			errCh := make(chan error)
			go res.Send(errCh)
			oldcmd.Run(oldReq, res)
			select {
			case err := <-errCh:
				if err != nil {
					select {
					case <-req.Context().Done():
						err = cmdkit.Error{Message: req.Context().Err().Error(), Code: cmdkit.ErrNormal}
					default:
					}

					if e, ok := err.(*cmdkit.Error); ok {
						err = *e
					}

					if e, ok := err.(cmdkit.Error); ok {
						re.SetError(e.Message, e.Code)
					} else {
						re.SetError(err.Error(), cmdkit.ErrNormal)
					}
				}
			case <-req.Context().Done():
				re.SetError(req.Context().Err(), cmdkit.ErrNormal)
			}
		}
	}

	if oldcmd.PreRun != nil {
		cmd.PreRun = func(req Request) error {
			oldReq := &requestWrapper{req}
			return oldcmd.PreRun(oldReq)
		}
	}

	cmd.Encoders = make(EncoderMap)

	for encType, m := range oldcmd.Marshalers {
		cmd.Encoders[EncodingType(encType)] = func(m oldcmds.Marshaler, encType oldcmds.EncodingType) func(req Request) func(io.Writer) Encoder {
			return func(req Request) func(io.Writer) Encoder {
				return func(w io.Writer) Encoder {
					return NewMarshalerEncoder(req, m, w)
				}
			}
		}(m, encType)
	}

	return cmd
}

// OldContext returns an oldcmds.Context from a Context
func OldContext(ctx Context) oldcmds.Context {
	node, err := ctx.GetNode()

	oldCtx := oldcmds.Context{
		Online:     ctx.Online,
		ConfigRoot: ctx.ConfigRoot,
		LoadConfig: func(path string) (*config.Config, error) {
			return node.Repo.Config()
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, err
		},
	}

	oldCtx.ReqLog = OldReqLog(ctx.ReqLog)

	return oldCtx
}

// NewContext returns a Context from an oldcmds.Context
func NewContext(ctx oldcmds.Context) Context {
	node, err := ctx.GetNode()

	return Context{
		Online:     ctx.Online,
		ConfigRoot: ctx.ConfigRoot,
		LoadConfig: func(path string) (*config.Config, error) {
			return node.Repo.Config()
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, err
		},
	}
}

// OldReqLog returns an oldcmds.ReqLog from a ReqLog
func OldReqLog(newrl *ReqLog) *oldcmds.ReqLog {
	if newrl == nil {
		return nil
	}

	rl := &oldcmds.ReqLog{}

	for _, rle := range newrl.Requests {
		oldrle := &oldcmds.ReqLogEntry{
			StartTime: rle.StartTime,
			EndTime:   rle.EndTime,
			Active:    rle.Active,
			Command:   rle.Command,
			Options:   rle.Options,
			Args:      rle.Args,
			ID:        rle.ID,
		}
		rl.AddEntry(oldrle)
	}

	return rl
}
