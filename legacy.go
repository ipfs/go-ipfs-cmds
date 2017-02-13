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

// fakeResponse gives you a Response when you give it a ResponseEmitter
type fakeResponse struct {
	req oldcmds.Request
	re  ResponseEmitter
	out interface{}
}

func (r *fakeResponse) Send() error {
	if r.out == nil {
		return nil
	}

	if ch, ok := r.out.(chan interface{}); ok {
		r.out = <-chan interface{}(ch)
	}

	switch out := r.out.(type) {
	case <-chan interface{}:
		for x := range out {
			if err := r.re.Emit(x); err != nil {
				return err
			}
		}
	default:
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
}

func (mer *marshalerEncoderResponse) Output() interface{} {
	return mer.value
}

// make an Encoder from a Marshaler
type MarshalerEncoder struct {
	m   oldcmds.Marshaler
	req *oldcmds.Request
	w   io.Writer
}

func (me *MarshalerEncoder) Encode(v interface{}) error {
	res := &marshalerEncoderResponse{
		Response: oldcmds.NewResponse(*me.req),
		value:    v,
	}

	r, err := me.m(res)
	if err != nil {
		return err
	}

	_, err = io.Copy(me.w, r)
	return err
}

type wrappedResponseEmitter struct {
	r oldcmds.Response
}

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
	var req_ oldcmds.Request

	cmd = &Command{
		Options:   oldcmd.Options,
		Arguments: oldcmd.Arguments,
		Helptext:  oldcmd.Helptext,
		External:  oldcmd.External,
		Type:      oldcmd.Type,

		Run: func(req Request, re ResponseEmitter) {
			oldReq := &requestWrapper{req}
			res := &fakeResponse{req: oldReq, re: re}
			req_ = oldReq

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

	for encType, m := range oldcmd.Marshalers {
		cmd.Encoders = make(map[EncodingType]func(io.Writer) Encoder)
		cmd.Encoders[EncodingType(encType)] = func(w io.Writer) Encoder {
			log.Debugf("adding marshalerencoder for %v: %v; req: %v", encType, m, req_)
			return &MarshalerEncoder{
				req: &req_,
				m:   m,
				w:   w,
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
