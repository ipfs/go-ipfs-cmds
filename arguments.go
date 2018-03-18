package cmds

import (
	"bufio"
	"io"
)

// StdinArguments is used to iterate through arguments piped through stdin.
type StdinArguments interface {
	io.ReadCloser
	// Returns the next argument passed via stdin.
	//
	// This method will never return an error along with a value, it will
	// return one or the other.
	//
	// Once all arguments have been read, it will return "", io.EOF
	Next() (string, error)
}

type arguments struct {
	reader *bufio.Reader
	closer io.Closer
}

func newArguments(r io.ReadCloser) *arguments {
	return &arguments{
		reader: bufio.NewReader(r),
		closer: r,
	}
}

// Read implements the io.Reader interface
func (a *arguments) Read(b []byte) (int, error) {
	return a.reader.Read(b)
}

// Close implements the io.Closer interface
func (a *arguments) Close() error {
	return a.closer.Close()
}

// WriteTo implements the io.WriterTo interface
func (a *arguments) WriteTo(w io.Writer) (int64, error) {
	return a.reader.WriteTo(w)
}

// Next returns the next argument
func (a *arguments) Next() (string, error) {
	s, err := a.reader.ReadString('\n')
	switch err {
	case io.EOF:
		if s == "" {
			return "", io.EOF
		}
		// drop the error.
		return s, nil
	case nil:
		l := len(s)
		if l >= 2 && s[l-2] == '\r' {
			return s[:l-2], nil
		}
		return s[:l-1], nil
	default:
		return "", err
	}
}
