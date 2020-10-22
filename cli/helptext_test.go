package cli

import (
	"bytes"
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
	if strings.Contains(syn, "For more information about each command") {
		t.Fatal("Synopsis should not contain subcommands")
	}
}

func TestShortHelp(t *testing.T) {
	// ShortHelp behaves differently depending on whether the command is the root or not.
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"ls": {
				Helptext: cmds.HelpText{
					ShortDescription: `
				Displays the contents of an IPFS or IPNS object(s) at the given path.
				`},
			},
		},
	}
	// Ask for the help text for the ls command which has no subcommands
	path := []string{"ls"}
	buf := new(bytes.Buffer)
	ShortHelp("ipfs", root, path, buf)
	helpText := buf.String()
	t.Logf("Short help text: %s", helpText)
	if strings.Contains(helpText, "For more information about each command") {
		t.Fatal("ShortHelp should not contain subcommand info")
	}
}

func TestLongHelp(t *testing.T) {
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"ls": {
				Helptext: cmds.HelpText{
					ShortDescription: `
				Displays the contents of an IPFS or IPNS object(s) at the given path.
				`},
			},
		},
	}
	path := []string{"ls"}
	buf := new(bytes.Buffer)
	LongHelp("ipfs", root, path, buf)
	helpText := buf.String()
	t.Logf("Long help text: %s", helpText)
	if strings.Contains(helpText, "For more information about each command") {
		t.Fatal("LongHelp should not contain subcommand info")
	}
}
