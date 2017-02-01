package cmds

import (
	"io"
	"os"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	// TODO should be drop that?
	Request() *Request

	Error() *cmdsutil.Error
	Length() uint64

	Next() (interface{}, error)
}

/*
// FakeResponse returns a Response compatible to the old packet
func FakeResponse(re ResponseEmitter) Response {
	return &fakeResponse{re: re}
}
*/

type fakeResponse struct {
	re  ResponseEmitter
	out interface{}
}

func (r *fakeResponse) Request() Request {
	return nil
}

func (r *fakeResponse) SetError(err error, code ErrorType) {
	r.re.SetError(err, code)
}

func (r *fakeResponse) Error() *Error {
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

// FakeOldResponse returns a Response compatible to the old packet
func FakeOldResponse(re ResponseEmitter) oldcmds.Response {
	return &fakeOldResponse{&fakeResponse{re: re}}
}

type fakeOldResponse struct {
	*fakeResponse
}

func (r *fakeOldResponse) Request() oldcmds.Request {
	return nil
}

func (r *fakeOldResponse) SetError(err error, code oldcmds.ErrorType) {
	r.re.SetError(err, ErrorType(code))
}

func (r *fakeOldResponse) Error() *oldcmds.Error {
	return nil
}
