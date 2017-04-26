package cli

import (
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs-cmds"
	"gx/ipfs/Qmf7G7FikwUsm48Jm4Yw4VBGNZuyRaAMzpWDJcW8V71uV2/go-ipfs-cmdkit"
)

func TestSynopsisGenerator(t *testing.T) {
	command := &cmds.Command{
		Arguments: []cmdsutil.Argument{
			cmdsutil.StringArg("required", true, false, ""),
			cmdsutil.StringArg("variadic", false, true, ""),
		},
		Options: []cmdsutil.Option{
			cmdsutil.StringOption("opt", "o", "Option"),
		},
		Helptext: cmdsutil.HelpText{
			SynopsisOptionsValues: map[string]string{
				"opt": "OPTION",
			},
		},
	}
	syn := generateSynopsis(command, "cmd")
	t.Logf("Synopsis is: %s", syn)
	if !strings.HasPrefix(syn, "cmd ") {
		t.Fatal("Synopsis should start with command name")
	}
	if !strings.Contains(syn, "[--opt=<OPTION> | -o]") {
		t.Fatal("Synopsis should contain option descriptor")
	}
	if !strings.Contains(syn, "<required>") {
		t.Fatal("Synopsis should contain required argument")
	}
	if !strings.Contains(syn, "<variadic>...") {
		t.Fatal("Synopsis should contain variadic argument")
	}
	if !strings.Contains(syn, "[<variadic>...]") {
		t.Fatal("Synopsis should contain optional argument")
	}
	if !strings.Contains(syn, "[--]") {
		t.Fatal("Synopsis should contain options finalizer")
	}
}
