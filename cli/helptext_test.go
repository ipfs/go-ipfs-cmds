package cli

import (
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestSynopsisGenerator(t *testing.T) {
	command := &cmds.Command{
		Arguments: []cmds.Argument{
			cmds.StringArg("required", true, false, ""),
			cmds.StringArg("variadic", false, true, ""),
		},
		Options: []cmds.Option{
			cmds.StringOption("opt", "o", "Option"),
			cmds.StringsOption("var-opt", "Variadic Option"),
		},
		Helptext: cmds.HelpText{
			SynopsisOptionsValues: map[string]string{
				"opt": "OPTION",
			},
		},
	}
	terminalWidth := 100
	syn := generateSynopsis(terminalWidth, command, "cmd")
	t.Logf("Synopsis is: %s", syn)
	if !strings.HasPrefix(syn, "cmd ") {
		t.Fatal("Synopsis should start with command name")
	}
	if !strings.Contains(syn, "[--opt=<OPTION> | -o]") {
		t.Fatal("Synopsis should contain option descriptor")
	}
	if !strings.Contains(syn, "[--var-opt=<var-opt>]...") {
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
