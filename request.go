package cmds

import (
	"bufio"
	"context"
	"io"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmdkit/files"
)

// Request represents a call to a command from a consumer
type Request struct {
	Context context.Context
	Command *Command
	
	Path []string
	Arguments []string
	Options cmdkit.OptMap
	
	Body io.Reader
	Files files.File
}

func VarArgs(req Request, f func(string) error) error {
	if len(req.Arguments) >= len(req.Command.Arguments) {
		for _, arg := range req.Arguments[len(req.Command.Arguments)-1:] {
			err := f(arg)
			if err != nil {
				return err
			}
		}

		return nil
	}
	
	if req.Files == nil {
		log.Warning("expected more arguments from stdin")
		return nil
	}

	fi, err := req.Files.NextFile()
	if err != nil {
		return err
	}

	var any bool
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		any = true
		err := f(scan.Text())
		if err != nil {
			return err
		}
	}
	if !any {
		return f("")
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
