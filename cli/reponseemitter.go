package cli

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

type ErrSet struct {
	error
}

func NewResponseEmitter(w io.WriteCloser, enc func(cmds.Request) func(io.Writer) cmds.Encoder, req cmds.Request) (cmds.ResponseEmitter, <-chan *cmdsutil.Error) {
	ch := make(chan *cmdsutil.Error)

	if enc == nil {
		enc = func(cmds.Request) func(io.Writer) cmds.Encoder {
			return func(io.Writer) cmds.Encoder {
				return nil
			}
		}
	}

	return &responseEmitter{w: w, enc: enc(req)(w), ch: ch}, ch
}

type responseEmitter struct {
	w io.WriteCloser

	length uint64
	err    *cmdsutil.Error
	enc    cmds.Encoder

	tees []cmds.ResponseEmitter

	emitted bool
	ch      chan<- *cmdsutil.Error
}

func (re *responseEmitter) SetLength(l uint64) {
	re.length = l

	for _, re_ := range re.tees {
		re_.SetLength(l)
	}
}

func (re *responseEmitter) SetError(v interface{}, errType cmdsutil.ErrorType) {
	log.Debugf("re.SetError(%v, %v)", v, errType)

	err := &cmdsutil.Error{Message: fmt.Sprint(v), Code: errType}
	re.Emit(err)

	for _, re_ := range re.tees {
		re_.SetError(v, errType)
	}
}

func (re *responseEmitter) Close() error {
	if re.w == nil {
		log.Warning("more than one call to RespEm.Close!")
		return nil
	}

	log.Debug("closing RE")
	log.Debugf("%s", debug.Stack())
	close(re.ch)
	re.w = nil

	return nil
}

// Head returns the current head.
// TODO: maybe it makes sense to make these pointers to shared memory?
//   might not be so clever though...concurrency and stuff
func (re *responseEmitter) Head() cmds.Head {
	return cmds.Head{
		Len: re.length,
		Err: re.err,
	}
}

func (re *responseEmitter) Emit(v interface{}) error {
	if v == nil {
		log.Debug(string(debug.Stack()))
	}
	log.Debugf("re.Emit(%T)", v)

	if re.w == nil {
		return io.ErrClosedPipe
	}

	if err, ok := v.(cmdsutil.Error); ok {
		log.Warningf("fixerr %s", debug.Stack())
		v = &err
	}

	if err, ok := v.(*cmdsutil.Error); ok {
		log.Warning("sending err to ch")
		log.Debugf("%s", debug.Stack())
		re.ch <- err
		log.Debug("sent err to ch")
		re.Close()
		return nil
	}

	log.Debug("copying to tees")
	for _, re_ := range re.tees {
		go re_.Emit(v)
	}
	log.Debug("done")

	var err error

	switch t := v.(type) {
	case io.Reader:
		var n int64

		log.Debug("case reader")
		log.Debug("start copying received reader to cli")
		n, err = io.Copy(re.w, t)
		log.Debug("done copying received reader to cli, n=", n)
	default:
		log.Debug("case default")
		if re.enc != nil {
			log.Debug("using encoder")
			err = re.enc.Encode(v)
		} else {
			log.Debug("using fprintln")
			_, err = fmt.Fprintln(re.w, t)
		}
	}

	return err
}

func (re *responseEmitter) Tee(re_ cmds.ResponseEmitter) {
	re.tees = append(re.tees, re_)

	if re.emitted {
		re_.SetLength(re.length)
	}

	if re.err != nil {
		re_.SetError(re.err.Message, re.err.Code)
	}
}
