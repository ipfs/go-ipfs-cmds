package http

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs-cmds"
	"gx/ipfs/Qmf7G7FikwUsm48Jm4Yw4VBGNZuyRaAMzpWDJcW8V71uV2/go-ipfs-cmdkit"
)

type Response struct {
	length uint64
	err    *cmdsutil.Error

	res *http.Response
	req cmds.Request

	rr  *responseReader
	dec cmds.Decoder
}

func (res *Response) Request() cmds.Request {
	return res.req
}

func (res *Response) Error() *cmdsutil.Error {
	return res.err
}

func (res *Response) Length() uint64 {
	return res.length
}

func (res *Response) Next() (v interface{}, err error) {
	if res.err != nil {
		return nil, cmds.ErrRcvdError
	}

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

	var (
		t = reflect.TypeOf(res.req.Command().Type)
	)

	if t != nil {
		// reflection worked, decode into proper type
		v = reflect.New(t).Interface()
		err = res.dec.Decode(v)
	} else {
		// reflection didn't work, decode into empty interface
		err = res.dec.Decode(&v)
	}

	if err != nil {
		rxErr := res.res.Trailer.Get(StreamErrHeader)
		if err.Error() == rxErr {
			res.err = &cmdsutil.Error{Message: rxErr, Code: cmdsutil.ErrNormal}
			return nil, cmds.ErrRcvdError
		}
	}

	return v, err
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

	// If we ran into an error
	if httpRes.StatusCode >= http.StatusBadRequest {
		e := &cmdsutil.Error{}

		switch {
		case httpRes.StatusCode == http.StatusNotFound:
			// handle 404s
			e.Message = "Command not found."
			e.Code = cmdsutil.ErrClient

		case contentType == plainText:
			// handle non-marshalled errors
			mes, err := ioutil.ReadAll(res.rr)
			if err != nil {
				return nil, err
			}
			e.Message = string(mes)
			e.Code = cmdsutil.ErrNormal

		default:
			// handle marshalled errors
			err = res.dec.Decode(&e)
			if err != nil {
				return nil, err
			}
		}

		e.Message = strings.Trim(e.Message, "\n\r\t ")

		res.err = e

		return res, nil
	}

	if contentType != applicationJson {
		// nil decoder means stream
		return res, nil

	} else if len(httpRes.Header.Get(channelHeader)) > 0 {
		encTypeStr, found, err := req.Option(cmdsutil.EncShort).String()
		if err != nil {
			return nil, err
		}

		encType := cmds.EncodingType(encTypeStr)

		if !found || len(encType) == 0 {
			encType = cmds.JSON
		}

		res.dec = cmds.Decoders[encType](res.rr)

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
