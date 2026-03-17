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
	syn := generateSynopsis(terminalWidth, command, "cmd", nil)
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

func newTestRootAndSub() (root *cmds.Command, sub *cmds.Command) {
	sub = &cmds.Command{
		Helptext: cmds.HelpText{
			Tagline: "Add a file",
		},
		Options: []cmds.Option{
			cmds.BoolOption("pin", "p", "Pin the file"),
		},
		Arguments: []cmds.Argument{
			cmds.StringArg("file", true, false, "File to add"),
		},
	}
	root = &cmds.Command{
		Helptext: cmds.HelpText{
			Tagline: "Global root",
		},
		Options: []cmds.Option{
			cmds.StringOption("encoding", "enc", "Output encoding"),
			cmds.StringOption("timeout", "Max time"),
		},
		Subcommands: map[string]*cmds.Command{
			"add": sub,
		},
	}
	return root, sub
}

func TestLongHelpGlobalOptions(t *testing.T) {
	root, _ := newTestRootAndSub()

	var buf bytes.Buffer
	if err := LongHelp("ipfs", root, []string{"add"}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	t.Log(out)

	if !strings.Contains(out, "OPTIONS") {
		t.Fatal("long help should contain OPTIONS section")
	}
	if !strings.Contains(out, "--pin") {
		t.Fatal("long help OPTIONS should contain subcommand option --pin")
	}
	if !strings.Contains(out, "GLOBAL OPTIONS") {
		t.Fatal("long help should contain GLOBAL OPTIONS section")
	}
	if !strings.Contains(out, "--encoding") {
		t.Fatal("GLOBAL OPTIONS should contain --encoding")
	}
	if !strings.Contains(out, "--timeout") {
		t.Fatal("GLOBAL OPTIONS should contain --timeout")
	}
}

func TestLongHelpRootNoGlobalSection(t *testing.T) {
	root, _ := newTestRootAndSub()

	var buf bytes.Buffer
	if err := LongHelp("ipfs", root, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if strings.Contains(out, "GLOBAL OPTIONS") {
		t.Fatal("root long help should NOT contain GLOBAL OPTIONS section")
	}
}

func TestShortHelpGlobalOptionsHint(t *testing.T) {
	root, _ := newTestRootAndSub()

	var buf bytes.Buffer
	if err := ShortHelp("ipfs", root, []string{"add"}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	t.Log(out)

	if !strings.Contains(out, "Use 'ipfs --help' for global options.") {
		t.Fatal("short help should contain global options hint")
	}
}

func TestShortHelpRootNoHint(t *testing.T) {
	root, _ := newTestRootAndSub()

	var buf bytes.Buffer
	if err := ShortHelp("ipfs", root, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if strings.Contains(out, "global options") {
		t.Fatal("root short help should NOT contain global options hint")
	}
}

func TestSynopsisIncludesGlobalOptions(t *testing.T) {
	root, sub := newTestRootAndSub()
	globalOpts := collectGlobalOptions(root, []string{"add"}, sub)

	syn := generateSynopsis(120, sub, "ipfs add", globalOpts)
	t.Log(syn)

	if !strings.Contains(syn, "--pin") {
		t.Fatal("synopsis should contain subcommand option --pin")
	}
	if !strings.Contains(syn, "--encoding") {
		t.Fatal("synopsis should contain global option --encoding")
	}
	if !strings.Contains(syn, "--timeout") {
		t.Fatal("synopsis should contain global option --timeout")
	}
}

func TestCollectGlobalOptionsDedup(t *testing.T) {
	// If the subcommand redefines an option name from the root, it should
	// not appear in the global options list.
	sub := &cmds.Command{
		Options: []cmds.Option{
			cmds.StringOption("encoding", "Override encoding"),
		},
	}
	root := &cmds.Command{
		Options: []cmds.Option{
			cmds.StringOption("encoding", "enc", "Output encoding"),
			cmds.StringOption("timeout", "Max time"),
		},
		Subcommands: map[string]*cmds.Command{
			"sub": sub,
		},
	}

	globalOpts := collectGlobalOptions(root, []string{"sub"}, sub)
	for _, opt := range globalOpts {
		if opt.Name() == "encoding" {
			t.Fatal("global options should not include option already on leaf command")
		}
	}
	if len(globalOpts) != 1 || globalOpts[0].Name() != "timeout" {
		t.Fatalf("expected only timeout in global opts, got %v", globalOpts)
	}
}
