package cmds

import (
	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

// NewTeeEmitter creates a new ResponseEmitter.
// Writing to it will write to both the passed ResponseEmitters.
func NewTeeEmitter(re1, re2 ResponseEmitter) ResponseEmitter {
	return &teeEmitter{re1, re2}
}

type teeEmitter struct {
	ResponseEmitter

	re ResponseEmitter
}

func (re *teeEmitter) Close() error {
	err1 := re.ResponseEmitter.Close()
	err2 := re.re.Close()

	if err1 != nil {
		return err1
	}

	// XXX we drop the second error if both fail
	return err2
}

func (re *teeEmitter) Emit(v interface{}) error {
	err1 := re.ResponseEmitter.Emit(v)
	err2 := re.re.Emit(v)

	if err1 != nil {
		return err1
	}

	// XXX we drop the second error if both fail
	return err2
}

func (re *teeEmitter) SetLength(l uint64) {
	re.ResponseEmitter.SetLength(l)
	re.re.SetLength(l)
}

func (re *teeEmitter) SetError(err interface{}, code cmdsutil.ErrorType) {
	re.ResponseEmitter.SetError(err, code)
	re.re.SetError(err, code)
}
