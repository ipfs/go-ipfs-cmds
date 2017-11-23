package cmds

import (
	"bufio"
	"context"
	"fmt"
	"reflect"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmdkit/files"
)

// Request represents a call to a command from a consumer
type Request struct {
	Context       context.Context
	Root, Command *Command

	Path      []string
	Arguments []string
	Options   cmdkit.OptMap

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
		Root:      root,
		Command:   cmd,
		Context:   ctx,
	}

	return req, req.convertOptions(root)
}

// BodyArgs returns a scanner that returns arguments passed in the body as tokens.
func (req *Request) BodyArgs() (*bufio.Scanner, error) {
	if len(req.Arguments) >= len(req.Command.Arguments) {
		return nil, fmt.Errorf("all arguments covered by positional arguments")
	}

	if req.Files == nil {
		return nil, fmt.Errorf("expected more arguments from stdin")
	}

	fi, err := req.Files.NextFile()
	if err != nil {
		return nil, err
	}

	return bufio.NewScanner(fi), nil
}

func (req *Request) SetOption(name string, value interface{}) {
	optDefs, err := req.Root.GetOptions(req.Path)
	log.Debugf("req.Root.GetOptions returned %q", err)
	optDef, found := optDefs[name]

	// unknown option, simply set the value and return
	// TODO we might error out here instead
	if !found {
		req.Options[name] = value
		return
	}

	if cname, ok := optDef.CanonicalName(); ok {
		name = cname
	}

	req.Options[name] = value
	return
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
					value := fmt.Sprintf("value %q", v)
					if len(str) == 0 {
						value = "empty value"
					}
					return fmt.Errorf("Could not convert %q to type %q (for option %q)",
						value, opt.Type().String(), "-"+k)
				}
				req.Options[k] = val

			} else {
				return fmt.Errorf("Option %q should be type %q, but got type %q",
					k, opt.Type().String(), kind.String())
			}
		}

		for _, name := range opt.Names() {
			if _, ok := req.Options[name]; name != k && ok {
				return fmt.Errorf("Duplicate command options were provided (%q and %q)",
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
