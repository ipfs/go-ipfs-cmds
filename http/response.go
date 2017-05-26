package http

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs-cmds"
	"gx/ipfs/QmeGapzEYCQkoEYN5x5MCPdj1zMGMHRjcPbA26sveo2XV4/go-ipfs-cmdkit"
)

type Response struct {
	length uint64
	err    *cmdkit.Error

	res *http.Response
	req cmds.Request

	rr  *responseReader
	dec cmds.Decoder
}

func (res *Response) Request() cmds.Request {
	return res.req
}

func (res *Response) Error() *cmdkit.Error {
	e := res.err
	res.err = nil
	return e
}

func (res *Response) Length() uint64 {
	return res.length
}

func (res *Response) Next() (interface{}, error) {

	// nil decoder means stream not chunks
	// but only do that once
	if res.dec == nil {
		if res.rr == nil {
			return nil, io.EOF
		} else {
			rr := res.rr
			res.rr = nil
			return rr, nil
		}
	}

	a := &cmds.Any{}
	a.Add(&cmdkit.Error{})
	a.Add(res.req.Command().Type)

	err := res.dec.Decode(a)

	// last error was sent as value, now we get the same error from the headers. ignore and EOF!
	if err != nil && res.err != nil && err.Error() == res.err.Error() {
		err = io.EOF
	}
	if err != nil {
		return nil, err
	}

	return a.Interface(), err
}

// getResponse decodes a http.Response to create a cmds.Response
func getResponse(httpRes *http.Response, req cmds.Request) (cmds.Response, error) {
	var err error
	res := &Response{
		res: httpRes,
		req: req,
		rr:  &responseReader{httpRes},
	}

	lengthHeader := httpRes.Header.Get(extraContentLengthHeader)
	if len(lengthHeader) > 0 {
		length, err := strconv.ParseUint(lengthHeader, 10, 64)
		if err != nil {
			return nil, err
		}
		res.length = length
	}

	contentType := httpRes.Header.Get(contentTypeHeader)
	contentType = strings.Split(contentType, ";")[0]

	if len(httpRes.Header.Get(channelHeader)) > 0 {
		encTypeStr, found, err := req.Option(cmdkit.EncShort).String()
		if err != nil {
			return nil, err
		}

		encType := cmds.EncodingType(encTypeStr)

		if found {
			res.dec = cmds.Decoders[encType](res.rr)
		}
	}

	// If we ran into an error
	if httpRes.StatusCode >= http.StatusBadRequest {
		e := &cmdkit.Error{}

		switch {
		case httpRes.StatusCode == http.StatusNotFound:
			// handle 404s
			e.Message = "Command not found."
			e.Code = cmdkit.ErrClient

		case contentType == plainText:
			// handle non-marshalled errors
			mes, err := ioutil.ReadAll(res.rr)
			if err != nil {
				return nil, err
			}
			e.Message = string(mes)
			e.Code = cmdkit.ErrNormal

		default:
			// handle marshalled errors
			err = res.dec.Decode(&e)
			if err != nil {
				return nil, err
			}
		}

		res.err = e

		return res, nil
	}

	return res, nil
}

// responseReader reads from the response body, and checks for an error
// in the http trailer upon EOF, this error if present is returned instead
// of the EOF.
type responseReader struct {
	resp *http.Response
}

func (r *responseReader) Read(b []byte) (int, error) {
	if r == nil || r.resp == nil {
		return 0, io.EOF
	}

	n, err := r.resp.Body.Read(b)

	// reading on a closed response body is as good as an io.EOF here
	if err != nil && strings.Contains(err.Error(), "read on closed response body") {
		err = io.EOF
	}
	if err == io.EOF {
		_ = r.resp.Body.Close()
		trailerErr := r.checkError()
		if trailerErr != nil {
			return n, trailerErr
		}
	}
	return n, err
}

func (r *responseReader) checkError() error {
	if e := r.resp.Trailer.Get(StreamErrHeader); e != "" {
		return errors.New(e)
	}
	return nil
}

func (r *responseReader) Close() error {
	return r.resp.Body.Close()
}
