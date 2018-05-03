package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
)

var _ ResponseEmitter = &responseEmitter{}

func NewResponseEmitter(stdout, stderr io.Writer, enc func(*cmds.Request) func(io.Writer) cmds.Encoder, req *cmds.Request) (cmds.ResponseEmitter, <-chan int) {
	ch := make(chan int)
	encType := cmds.GetEncoding(req)

	if enc == nil {
		enc = func(*cmds.Request) func(io.Writer) cmds.Encoder {
			return func(io.Writer) cmds.Encoder {
				return nil
			}
		}
	}

	return &responseEmitter{
		stdout:  stdout,
		stderr:  stderr,
		encType: encType,
		enc:     enc(req)(stdout),
		ch:      ch,
	}, ch
}

// ResponseEmitter extends cmds.ResponseEmitter to give better control over the command line
type ResponseEmitter interface {
	cmds.ResponseEmitter

	Stdout() io.Writer
	Stderr() io.Writer
	Exit(int)
}

type responseEmitter struct {
	l      sync.Mutex
	stdout io.Writer
	stderr io.Writer

	length  uint64
	err     *cmdkit.Error // TODO do we really need this?
	enc     cmds.Encoder
	encType cmds.EncodingType
	exit    int
	closed  bool

	errOccurred bool

	ch chan<- int
}

func (re *responseEmitter) Type() cmds.PostRunType {
	return cmds.CLI
}

func (re *responseEmitter) SetLength(l uint64) {
	re.length = l
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	re.enc = enc(re.stdout)
}

func (re *responseEmitter) CloseWithError(err error) error {
	e, ok := err.(*cmdkit.Error)
	if !ok {
		e = &cmdkit.Error{
			Message: err.Error(),
		}
	}

	re.l.Lock()
	defer re.l.Unlock()

	re.errOccurred = true
	re.exit = 1 // TODO we could let err carry an exit code
	re.err = e

	_, err = fmt.Fprintln(re.stderr, "Error:", e.Message)
	if err != nil {
		return err
	}

	return re.close()
}

func (re *responseEmitter) isClosed() bool {
	re.l.Lock()
	defer re.l.Unlock()

	return re.closed
}

func (re *responseEmitter) Close() error {
	re.l.Lock()
	defer re.l.Unlock()

	return re.close()
}

func (re *responseEmitter) close() error {
	if re.closed {
		return errors.New("closing closed responseemitter")
	}

	re.ch <- re.exit
	close(re.ch)

	defer func() {
		re.stdout = nil
		re.stderr = nil
		re.closed = true
	}()

	if f, ok := re.stderr.(*os.File); ok {
		err := f.Sync()
		if err != nil {
			return err
		}
	}
	if f, ok := re.stdout.(*os.File); ok {
		err := f.Sync()
		if err != nil {
			return err
		}
	}

	return nil
}

// Head returns the current head.
// TODO: maybe it makes sense to make these pointers to shared memory?
//   might not be so clever though...concurrency and stuff
// TODO: can we maybe drop this function? Then we could also remove the err struct field
func (re *responseEmitter) Head() cmds.Head {
	return cmds.Head{
		Len: re.length,
		Err: re.err,
	}
}

func (re *responseEmitter) Emit(v interface{}) error {
	// unwrap
	if val, ok := v.(cmds.Single); ok {
		v = val.Value
	}

	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}

	// TODO find a better solution for this.
	// Idea: use the actual cmd.Type and not *cmd.Type
	// would need to fix all commands though
	switch c := v.(type) {
	case *string:
		v = *c
	case *int:
		v = *c
	}

	if ch, isChan := v.(<-chan interface{}); isChan {
		log.Debug("iterating over chan...", ch)
		for v = range ch {
			err := re.Emit(v)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if re.isClosed() {
		return io.ErrClosedPipe
	}

	var err error

	switch t := v.(type) {
	case *cmdkit.Error:
		log.Errorf("got an error: %v\n%s", err, debug.Stack())
	case io.Reader:
		_, err = io.Copy(re.stdout, t)
		if err != nil {
			return err
		}
	default:
		if re.enc != nil {
			err = re.enc.Encode(v)
		} else {
			_, err = fmt.Fprintln(re.stdout, t)
		}
	}

	return err
}

// Stderr returns the ResponseWriter's stderr
func (re *responseEmitter) Stderr() io.Writer {
	return re.stderr
}

// Stdout returns the ResponseWriter's stdout
func (re *responseEmitter) Stdout() io.Writer {
	return re.stdout
}

// Exit sends code to the channel that was returned by NewResponseEmitter, so main() can pass it to os.Exit()
func (re *responseEmitter) Exit(code int) {
	defer re.Close()

	re.l.Lock()
	defer re.l.Unlock()
	re.exit = code
}
