package legacy

import (
	"io"

	"github.com/ipfs/go-ipfs-cmds"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("cmds/lgc")

// OldCommand returns an oldcmds.Command from a Command.
func OldCommand(cmd *cmds.Command) *oldcmds.Command {
	oldcmd := &oldcmds.Command{
		Options:   cmd.Options,
		Arguments: cmd.Arguments,
		Helptext:  cmd.Helptext,
		External:  cmd.External,
		Type:      cmd.Type,

		Subcommands: func() map[string]*oldcmds.Command {
			cs := make(map[string]*oldcmds.Command)

			/*
				for k, v := range cmd.OldSubcommands {
					cs[k] = v
				}
			*/

			for k, v := range cmd.Subcommands {
				cs[k] = OldCommand(v)
			}

			return cs
		}(),
	}

	if cmd.Run != nil {
		oldcmd.Run = func(oldReq oldcmds.Request, res oldcmds.Response) {
			req := FromOldRequest(oldReq)
			re := &wrappedResponseEmitter{res}

			cmd.Run(req, re, oldReq.InvocContext())
		}
	}
	if cmd.PreRun != nil {
		oldcmd.PreRun = func(oldReq oldcmds.Request) error {
			req := FromOldRequest(oldReq)
			return cmd.PreRun(req)
		}
	}

	return oldcmd
}

// NewCommand returns a Command from an oldcmds.Command
func NewCommand(oldcmd *oldcmds.Command) *cmds.Command {
	if oldcmd == nil {
		return nil
	}
	var cmd *cmds.Command

	cmd = &cmds.Command{
		Options:   oldcmd.Options,
		Arguments: oldcmd.Arguments,
		Helptext:  oldcmd.Helptext,
		External:  oldcmd.External,
		Type:      oldcmd.Type,

		// OldSubcommands: oldcmd.Subcommands,
	}

	if oldcmd.Run != nil {
		cmd.Run = func(req *cmds.Request, re cmds.ResponseEmitter, env interface{}) {
			oldReq := &requestWrapper{req, OldContext(env)}
			res := &fakeResponse{req: oldReq, re: re, wait: make(chan struct{})}

			errCh := make(chan error)
			go res.Send(errCh)
			oldcmd.Run(oldReq, res)
			err := <-errCh
			if err != nil {
				log.Error(err)
			}
		}
	}

	if oldcmd.PreRun != nil {
		cmd.PreRun = func(req *cmds.Request) error {
			oldReq := &requestWrapper{req, nil}
			return oldcmd.PreRun(oldReq)
		}
	}

	cmd.Encoders = make(cmds.EncoderMap)

	for encType, m := range oldcmd.Marshalers {
		cmd.Encoders[cmds.EncodingType(encType)] = func(m oldcmds.Marshaler, encType oldcmds.EncodingType) func(req *cmds.Request) func(io.Writer) cmds.Encoder {
			return func(req *cmds.Request) func(io.Writer) cmds.Encoder {
				return func(w io.Writer) cmds.Encoder {
					return NewMarshalerEncoder(req, m, w)
				}
			}
		}(m, encType)
	}

	return cmd
}
