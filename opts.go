package cmds

import (
	"github.com/ipfs/go-ipfs-cmdkit"
)

// Flag names
const (
	EncShort     = "enc"
	EncLong      = "encoding"
	RecShort     = "r"
	RecLong      = "recursive"
	ChanOpt      = "stream-channels"
	TimeoutOpt   = "timeout"
	OptShortHelp = "h"
	OptLongHelp  = "help"
	DerefLong    = "dereference-args"
	StdinName    = "stdin-name"
	Hidden       = "hidden"
	HiddenShort  = "H"
)

// options that are used by this package
var OptionEncodingType = cmdkit.StringOption(EncLong, EncShort, "The encoding type the output should be encoded with (json, xml, or text)").WithDefault("text")
var OptionRecursivePath = cmdkit.BoolOption(RecLong, RecShort, "Add directory paths recursively")
var OptionStreamChannels = cmdkit.BoolOption(ChanOpt, "Stream channel output")
var OptionTimeout = cmdkit.StringOption(TimeoutOpt, "Set a global timeout on the command")
var OptionDerefArgs = cmdkit.BoolOption(DerefLong, "Symlinks supplied in arguments are dereferenced")
var OptionStdinName = cmdkit.StringOption(StdinName, "Assign a name if the file source is stdin.")
var OptionHidden = cmdkit.BoolOption(Hidden, HiddenShort, "Include files that are hidden. Only takes effect on recursive add.")
