package cmds

import (
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/config"
)

type responseWrapper struct {
	Response

	out interface{}
}

func (rw *responseWrapper) Request() oldcmds.Request {
	return &requestWrapper{rw.Response.Request()}
}

func (rw *responseWrapper) Output() interface{} {
	log.Debug("rw.output()")
	log.Debugf("rw.Response is of type %T", rw.Response)
	if rw.out == nil {
		x, err := rw.Next()
		if err != nil {
			return nil
		}
		log.Debug("next returned ", x, err)

		if r, ok := x.(io.Reader); ok {
			rw.out = r
		} else {
			ch := make(chan interface{})
			rw.out = ch
			go func() {
				defer close(ch)
				ch <- x

				for {
					x, err := rw.Next()
					if err == io.EOF {
						return
					}
					if err != nil {
						log.Debug("unhandled error in call to Next()")
						return
					}

					ch <- x
				}
			}()
		}
	}

	return rw.out
}

func (rw *responseWrapper) SetError(error, cmdsutil.ErrorType) {}
func (rw *responseWrapper) SetOutput(interface{})              {}
func (rw *responseWrapper) SetLength(uint64)                   {}
func (rw *responseWrapper) SetCloser(io.Closer)                {}

func (rw *responseWrapper) Close() error { return nil }

func (rw *responseWrapper) Marshal() (io.Reader, error) { return nil, nil }
func (rw *responseWrapper) Reader() (io.Reader, error)  { return nil, nil }

func (rw *responseWrapper) Stdout() io.Writer { return os.Stdout }
func (rw *responseWrapper) Stderr() io.Writer { return os.Stderr }

func WrapOldRequest(r oldcmds.Request) Request {
	return &oldRequestWrapper{r}
}

// requestWrapper implements a oldcmds.Request from an Request
type requestWrapper struct {
	Request
}

func (r *requestWrapper) InvocContext() *oldcmds.Context {
	ctx := OldContext(*r.Request.InvocContext())
	return &ctx
}

func (r *requestWrapper) SetInvocContext(ctx oldcmds.Context) {
	r.Request.SetInvocContext(NewContext(ctx))
}

// TODO keks
func (r *requestWrapper) Command() *oldcmds.Command { return nil }

// oldRequestWrapper implements a Request from an oldcmds.Request
type oldRequestWrapper struct {
	oldcmds.Request
}

func (r *oldRequestWrapper) InvocContext() *Context {
	ctx := NewContext(*r.Request.InvocContext())
	return &ctx
}

func (r *oldRequestWrapper) SetInvocContext(ctx Context) {
	r.Request.SetInvocContext(OldContext(ctx))
}

// TODO keks
func (r *oldRequestWrapper) Command() *Command { return nil }

///

// fakeResponse gives you a oldcmds.Response when you give it a ResponseEmitter
type fakeResponse struct {
	req oldcmds.Request
	re  ResponseEmitter
	out interface{}
}

func (r *fakeResponse) Send() error {
	log.Debugf("fakeResponse: sending %v to RE of type %T", r.out, r.re)
	defer log.Debugf("fakeResponse: done")

	if r.out == nil {
		return nil
	}

	if ch, ok := r.out.(chan interface{}); ok {
		r.out = <-chan interface{}(ch)
	}

	switch out := r.out.(type) {
	case <-chan interface{}:
		for x := range out {
			log.Debugf("fakeResponse: calling Emit(%v) in loop", out)
			if err := r.re.Emit(x); err != nil {
				return err
			}
		}
	default:
		log.Debug("fakeResponse: calling Emit(%v) once", out)
		return r.re.Emit(out)
	}

	return nil
}

func (r *fakeResponse) Request() oldcmds.Request {
	return r.req
}

func (r *fakeResponse) SetError(err error, code cmdsutil.ErrorType) {
	r.re.SetError(err, code)
}

func (r *fakeResponse) Error() *cmdsutil.Error {
	return nil
}

func (r *fakeResponse) SetOutput(v interface{}) {
	r.out = v
}

func (r *fakeResponse) Output() interface{} {
	return r.out
}

func (r *fakeResponse) SetLength(l uint64) {
	r.re.SetLength(l)
}

func (r *fakeResponse) Length() uint64 {
	return 0
}

func (r *fakeResponse) Close() error {
	return r.re.Close()
}

func (r *fakeResponse) SetCloser(io.Closer) {}

func (r *fakeResponse) Reader() (io.Reader, error) {
	return nil, nil
}

func (r *fakeResponse) Marshal() (io.Reader, error) {
	return nil, nil
}

func (r *fakeResponse) Stdout() io.Writer {
	return os.Stdout
}

func (r *fakeResponse) Stderr() io.Writer {
	return os.Stderr
}

///

type marshalerEncoderResponse struct {
	oldcmds.Response // so we don't need to do the unimportant stuff

	value interface{}
	err   *cmdsutil.Error
}

func (mer *marshalerEncoderResponse) Output() interface{} {
	return mer.value
}

func (mer *marshalerEncoderResponse) Error() *cmdsutil.Error {
	return mer.err
}

// make an Encoder from a Marshaler
type MarshalerEncoder struct {
	m   oldcmds.Marshaler
	res oldcmds.Response
	w   io.Writer
}

func (me *MarshalerEncoder) Encode(v interface{}) error {
	log.Debugf("ME.Encode: me: %#v", me)
	r, err := me.m(me.res)
	log.Debugf("ME.Encode: r:%v, err:%v", r, err)
	if err != nil || r == nil {
		return err
	}

	_, err = io.Copy(me.w, r)
	return err
}

type wrappedResponseEmitter struct {
	r oldcmds.Response
}

func (re *wrappedResponseEmitter) Tee(re_ ResponseEmitter) {}

func (re *wrappedResponseEmitter) SetLength(l uint64) {
	re.r.SetLength(l)
}

func (re *wrappedResponseEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	re.r.SetError(fmt.Errorf("%v", err), code)
}

func (re *wrappedResponseEmitter) Close() error {
	return re.r.Close()
}

func (re *wrappedResponseEmitter) Emit(v interface{}) error {
	if re.r.Output() == nil {
		switch c := v.(type) {
		case io.Reader:
			re.r.SetOutput(c)
			return nil
		default:
			re.r.SetOutput(make(chan interface{}))
		}
	}

	re.r.Output().(chan interface{}) <- v

	return nil
}

func OldCommand(cmd *Command) *oldcmds.Command {
	oldcmd := &oldcmds.Command{
		Options:   cmd.Options,
		Arguments: cmd.Arguments,
		Helptext:  cmd.Helptext,
		External:  cmd.External,
		Type:      cmd.Type,

		Run: func(oldReq oldcmds.Request, res oldcmds.Response) {
			req := &oldRequestWrapper{oldReq}
			re := &wrappedResponseEmitter{res}

			cmd.Run(req, re)
		},
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

	if cmd.PreRun != nil {
		oldcmd.PreRun = func(oldReq oldcmds.Request) error {
			req := &oldRequestWrapper{oldReq}
			return cmd.PreRun(req)
		}
	}

	return oldcmd
}

func NewCommand(oldcmd *oldcmds.Command) *Command {
	if oldcmd == nil {
		return nil
	}
	var cmd *Command

	// XXX we'll set this as request inside the encoders and then copy it there later on
	cmd = &Command{
		Options:   oldcmd.Options,
		Arguments: oldcmd.Arguments,
		Helptext:  oldcmd.Helptext,
		External:  oldcmd.External,
		Type:      oldcmd.Type,

		Run: func(req Request, re ResponseEmitter) {
			oldReq := &requestWrapper{req}
			res := &fakeResponse{req: oldReq, re: re}

			oldcmd.Run(oldReq, res)

			res.Send()
		},

		OldSubcommands: oldcmd.Subcommands,
	}

	if oldcmd.PreRun != nil {
		cmd.PreRun = func(req Request) error {
			oldReq := &requestWrapper{req}
			return oldcmd.PreRun(oldReq)
		}
	}

	cmd.Encoders = make(map[EncodingType]func(Response) func(io.Writer) Encoder)

	for encType, m := range oldcmd.Marshalers {
		log.Debugf("adding marshaler %v for type encType %v", m, encType)
		cmd.Encoders[EncodingType(encType)] = func(res Response) func(io.Writer) Encoder {

			return func(w io.Writer) Encoder {
				log.Debugf("adding marshalerencoder for %v: %v; res: %v", encType, m, res)

				return &MarshalerEncoder{
					res: &responseWrapper{Response: res},
					m:   m,
					w:   w,
				}
			}
		}
	}

	return cmd
}

func OldContext(ctx Context) oldcmds.Context {
	node, err := ctx.GetNode()

	return oldcmds.Context{
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
