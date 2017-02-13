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

func NewResponseEmitter(w io.WriteCloser, enc func(io.Writer) cmds.Encoder) cmds.ResponseEmitter {
	if enc == nil {
		enc = func(io.Writer) cmds.Encoder { return nil }
	}

	return &responseEmitter{w: w, enc: enc(w)}
}

type responseEmitter struct {
	w io.WriteCloser

	length uint64
	err    *cmdsutil.Error
	enc    cmds.Encoder
}

func (re *responseEmitter) SetLength(l uint64) {
	re.length = l
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	re.enc = enc(re.w)
}

func (re *responseEmitter) SetError(v interface{}, errType cmdsutil.ErrorType) {
	log.Debug("re.SetError(%v, %v)", v, errType)

	err := &cmdsutil.Error{Message: fmt.Sprint(v), Code: errType}
	re.Emit(err)
	re.err = err
}

func (re *responseEmitter) Close() error {
	return re.w.Close()
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
	log.Debugf("v.T: %T, re.enc.T: %T, re.err.T: %T", v, re.enc, re.err)
	if re.err != nil {
		return ErrSet{re.err}
	}

	var err error

	switch t := v.(type) {
	case io.Reader:
		_, err = io.Copy(re.w, t)
	default:
		if re.enc != nil {
			err = re.enc.Encode(v)
		} else {
			_, err = fmt.Fprintln(re.w, t)
		}
	}

	return err
}
