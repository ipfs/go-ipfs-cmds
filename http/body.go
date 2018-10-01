package http

import "io"

// bodyWrapper wraps an io.Reader and calls onError whenever the Read function returns an error.
// This was designed for wrapping the request body, so we can know whether it was closed.
type bodyWrapper struct {
	io.ReadCloser
	onError func(err error)
}

func (bw bodyWrapper) Read(data []byte) (int, error) {
	n, err := bw.ReadCloser.Read(data)
	if err != nil {
		bw.onError(err)
	}

	return n, err
}
