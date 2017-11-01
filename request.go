package cmds

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmdkit/files"
)

// Request represents a call to a command from a consumer
type Request struct {
	Context context.Context
	Command *Command

	Path      []string
	Arguments []string
	Options   cmdkit.OptMap

	Body  io.Reader
	Files files.File
}

// NewRequest returns a request initialized with given arguments
// An non-nil error will be returned if the provided option values are invalid
func NewRequest(ctx context.Context, path []string, opts cmdkit.OptMap, args []string, file files.File, root *Command) (*Request, error) {
	if opts == nil {
		opts = make(cmdkit.OptMap)
	}

	cmd, err := root.Get(path)
	if err != nil {
		return nil, err
	}

	req := &Request{
		Path:      path,
		Options:   opts,
		Arguments: args,
		Files:     file,
		Command:   cmd,
		Context:   ctx,
		Body:      os.Stdin,
	}

	return req, req.convertOptions(root)
}

func (req *Request) convertOptions(root *Command) error {
	optDefs, err := root.GetOptions(req.Path)
	if err != nil {
		return err
	}

	for k, v := range req.Options {
		opt, ok := optDefs[k]
		if !ok {
			continue
		}

		kind := reflect.TypeOf(v).Kind()
		if kind != opt.Type() {
			if str, ok := v.(string); ok {
				val, err := opt.Parse(str)
				if err != nil {
					value := fmt.Sprintf("value '%v'", v)
					if len(str) == 0 {
						value = "empty value"
					}
					return fmt.Errorf("Could not convert %s to type '%s' (for option '-%s')",
						value, opt.Type().String(), k)
				}
				req.Options[k] = val

			} else {
				return fmt.Errorf("Option '%s' should be type '%s', but got type '%s'",
					k, opt.Type().String(), kind.String())
			}
		}

		for _, name := range opt.Names() {
			if _, ok := req.Options[name]; name != k && ok {
				return fmt.Errorf("Duplicate command options were provided ('%s' and '%s')",
					k, name)
			}
		}
	}

	return nil
}

// GetEncoding returns the EncodingType set in a request, falling back to JSON
func GetEncoding(req *Request) EncodingType {
	var (
		encType = EncodingType(Undefined)
		encStr  = string(Undefined)
		ok      = false
		opts    = req.Options
	)

	// try EncShort
	encIface := opts[cmdkit.EncShort]

	// if that didn't work, try EncLong
	if encIface == nil {
		encIface = opts[cmdkit.EncLong]
	}

	// try casting
	if encIface != nil {
		encStr, ok = encIface.(string)
	}

	// if casting worked, convert to EncodingType
	if ok {
		encType = EncodingType(encStr)
	}

	// in case of error, use default
	if !ok || encType == Undefined {
		encType = JSON
	}

	return encType
}
