package cli

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	files "github.com/ipfs/go-ipfs-files"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

type kvs map[string]interface{}
type words []string

func sameWords(a words, b words) bool {
	if len(a) != len(b) {
		return false
	}
	for i, w := range a {
		if w != b[i] {
			return false
		}
	}
	return true
}

func sameKVs(a kvs, b kvs) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if ks, ok := v.([]string); ok {
			bks, _ := b[k].([]string)
			for i := 0; i < len(ks); i++ {
				if ks[i] != bks[i] {
					return false
				}
			}
		} else if v != b[k] {
			return false
		}
	}
	return true
}

func TestSameWords(t *testing.T) {
	a := []string{"v1", "v2"}
	b := []string{"v1", "v2", "v3"}
	c := []string{"v2", "v3"}
	d := []string{"v2"}
	e := []string{"v2", "v3"}
	f := []string{"v2", "v1"}

	test := func(a words, b words, v bool) {
		if sameWords(a, b) != v {
			t.Errorf("sameWords('%v', '%v') != %v", a, b, v)
		}
	}

	test(a, b, false)
	test(a, a, true)
	test(a, c, false)
	test(b, c, false)
	test(c, d, false)
	test(c, e, true)
	test(b, e, false)
	test(a, b, false)
	test(a, f, false)
	test(e, f, false)
	test(f, f, true)
}

func testOptionHelper(t *testing.T, cmd *cmds.Command, args string, expectedOpts kvs, expectedWords words, expectErr bool) {
	req := &cmds.Request{}
	err := parse(req, strings.Split(args, " "), cmd)
	if err == nil {
		err = req.FillDefaults()
	}
	if expectErr {
		if err == nil {
			t.Errorf("Command line '%v' parsing should have failed", args)
		}
	} else if err != nil {
		t.Errorf("Command line '%v' failed to parse: %v", args, err)
	} else if !sameWords(req.Arguments, expectedWords) || !sameKVs(kvs(req.Options), expectedOpts) {
		t.Errorf("Command line '%v':\n  parsed as  %v %v\n  instead of %v %v",
			args, req.Options, req.Arguments, expectedOpts, expectedWords)
	}
}

func TestOptionParsing(t *testing.T) {
	cmd := &cmds.Command{
		Options: []cmds.Option{
			cmds.StringOption("string", "s", "a string"),
			cmds.StringOption("flag", "alias", "multiple long"),
			cmds.BoolOption("bool", "b", "a bool"),
			cmds.StringsOption("strings", "r", "strings array"),
			cmds.DelimitedStringsOption(",", "delimstrings", "d", "comma delimited string array"),
		},
		Subcommands: map[string]*cmds.Command{
			"test": &cmds.Command{},
			"defaults": &cmds.Command{
				Options: []cmds.Option{
					cmds.StringOption("opt", "o", "an option").WithDefault("def"),
				},
			},
		},
	}

	testFail := func(args string) {
		testOptionHelper(t, cmd, args, kvs{}, words{}, true)
	}

	test := func(args string, expectedOpts kvs, expectedWords words) {
		testOptionHelper(t, cmd, args, expectedOpts, expectedWords, false)
	}

	test("test -", kvs{}, words{"-"})
	testFail("-b -b")
	test("test beep boop", kvs{}, words{"beep", "boop"})
	testFail("-s")
	test("-s foo", kvs{"string": "foo"}, words{})
	test("-sfoo", kvs{"string": "foo"}, words{})
	test("-s=foo", kvs{"string": "foo"}, words{})
	test("-b", kvs{"bool": true}, words{})
	test("-bs foo", kvs{"bool": true, "string": "foo"}, words{})
	test("-sb", kvs{"string": "b"}, words{})
	test("-b test foo", kvs{"bool": true}, words{"foo"})
	test("--bool test foo", kvs{"bool": true}, words{"foo"})
	testFail("--bool=foo")
	testFail("--string")
	test("--string foo", kvs{"string": "foo"}, words{})
	test("--string=foo", kvs{"string": "foo"}, words{})
	test("-- -b", kvs{}, words{"-b"})
	test("test foo -b", kvs{"bool": true}, words{"foo"})
	test("-b=false", kvs{"bool": false}, words{})
	test("-b=true", kvs{"bool": true}, words{})
	test("-b=false test foo", kvs{"bool": false}, words{"foo"})
	test("-b=true test foo", kvs{"bool": true}, words{"foo"})
	test("--bool=true test foo", kvs{"bool": true}, words{"foo"})
	test("--bool=false test foo", kvs{"bool": false}, words{"foo"})
	test("-b test true", kvs{"bool": true}, words{"true"})
	test("-b test false", kvs{"bool": true}, words{"false"})
	test("-b=FaLsE test foo", kvs{"bool": false}, words{"foo"})
	test("-b=TrUe test foo", kvs{"bool": true}, words{"foo"})
	test("-b test true", kvs{"bool": true}, words{"true"})
	test("-b test false", kvs{"bool": true}, words{"false"})
	test("-b --string foo test bar", kvs{"bool": true, "string": "foo"}, words{"bar"})
	test("-b=false --string bar", kvs{"bool": false, "string": "bar"}, words{})
	test("--strings a --strings b", kvs{"strings": []string{"a", "b"}}, words{})

	test("--delimstrings a,b", kvs{"delimstrings": []string{"a", "b"}}, words{})
	test("--delimstrings=a,b", kvs{"delimstrings": []string{"a", "b"}}, words{})
	test("-d a,b", kvs{"delimstrings": []string{"a", "b"}}, words{})
	test("-d=a,b", kvs{"delimstrings": []string{"a", "b"}}, words{})
	test("-d=a,b -d c --delimstrings d", kvs{"delimstrings": []string{"a", "b", "c", "d"}}, words{})

	testFail("foo test")
	test("defaults", kvs{"opt": "def"}, words{})
	test("defaults -o foo", kvs{"opt": "foo"}, words{})

	test("--flag=foo", kvs{"flag": "foo"}, words{})
	test("--alias=foo", kvs{"flag": "foo"}, words{})
	testFail("--flag=bar --alias=foo")
	testFail("--alias=bar --flag=foo")

	testFail("--bad-flag")
	testFail("--bad-flag=")
	testFail("--bad-flag=xyz")
	testFail("-z")
	testFail("-zz--- --")
}

func TestDefaultOptionParsing(t *testing.T) {
	testPanic := func(f func()) {
		fnFinished := false
		defer func() {
			if r := recover(); fnFinished == true {
				panic(r)
			}
		}()
		f()
		fnFinished = true
		t.Error("expected panic")
	}

	testPanic(func() { cmds.StringOption("string", "s", "a string").WithDefault(0) })
	testPanic(func() { cmds.StringOption("string", "s", "a string").WithDefault(false) })
	testPanic(func() { cmds.StringOption("string", "s", "a string").WithDefault(nil) })
	testPanic(func() { cmds.StringOption("string", "s", "a string").WithDefault([]string{"foo"}) })
	testPanic(func() { cmds.StringsOption("strings", "a", "a string array").WithDefault(0) })
	testPanic(func() { cmds.StringsOption("strings", "a", "a string array").WithDefault(false) })
	testPanic(func() { cmds.StringsOption("strings", "a", "a string array").WithDefault(nil) })
	testPanic(func() { cmds.StringsOption("strings", "a", "a string array").WithDefault("foo") })
	testPanic(func() { cmds.StringsOption("strings", "a", "a string array").WithDefault([]bool{false}) })
	testPanic(func() { cmds.DelimitedStringsOption(",", "dstrings", "d", "delimited string array").WithDefault(0) })
	testPanic(func() { cmds.DelimitedStringsOption(",", "dstrs", "d", "delimited string array").WithDefault(false) })
	testPanic(func() { cmds.DelimitedStringsOption(",", "dstrings", "d", "delimited string array").WithDefault(nil) })
	testPanic(func() { cmds.DelimitedStringsOption(",", "dstrs", "d", "delimited string array").WithDefault("foo") })
	testPanic(func() { cmds.DelimitedStringsOption(",", "dstrs", "d", "delimited string array").WithDefault([]int{0}) })

	testPanic(func() { cmds.BoolOption("bool", "b", "a bool").WithDefault(0) })
	testPanic(func() { cmds.BoolOption("bool", "b", "a bool").WithDefault(1) })
	testPanic(func() { cmds.BoolOption("bool", "b", "a bool").WithDefault(nil) })
	testPanic(func() { cmds.BoolOption("bool", "b", "a bool").WithDefault([]bool{false}) })
	testPanic(func() { cmds.BoolOption("bool", "b", "a bool").WithDefault([]string{"foo"}) })

	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(int(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(int32(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(int64(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(uint64(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(uint32(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(float32(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(float64(0)) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault(nil) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault([]uint{0}) })
	testPanic(func() { cmds.UintOption("uint", "u", "a uint").WithDefault([]string{"foo"}) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(int(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(int32(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(int64(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(uint(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(uint32(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(float32(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(float64(0)) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(nil) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault([]uint64{0}) })
	testPanic(func() { cmds.Uint64Option("uint64", "v", "a uint64").WithDefault([]string{"foo"}) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(int32(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(int64(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(uint(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(uint32(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(uint64(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(float32(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(float64(0)) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault(nil) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault([]int{0}) })
	testPanic(func() { cmds.IntOption("int", "i", "an int").WithDefault([]string{"foo"}) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(int(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(int32(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(uint(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(uint32(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(uint64(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(float32(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(float64(0)) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault(nil) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault([]int64{0}) })
	testPanic(func() { cmds.Int64Option("int64", "j", "an int64").WithDefault([]string{"foo"}) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(int(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(int32(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(int64(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(uint(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(uint32(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(uint64(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(float32(0)) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault(nil) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault([]int{0}) })
	testPanic(func() { cmds.FloatOption("float", "f", "a float64").WithDefault([]string{"foo"}) })

	cmd := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"defaults": &cmds.Command{
				Options: []cmds.Option{
					cmds.StringOption("string", "s", "a string").WithDefault("foo"),
					cmds.StringsOption("strings1", "a", "a string array").WithDefault([]string{"foo"}),
					cmds.StringsOption("strings2", "b", "a string array").WithDefault([]string{"foo", "bar"}),
					cmds.DelimitedStringsOption(",", "dstrings1", "c", "a delimited string array").WithDefault([]string{"foo"}),
					cmds.DelimitedStringsOption(",", "dstrings2", "d", "a delimited string array").WithDefault([]string{"foo", "bar"}),

					cmds.BoolOption("boolT", "t", "a bool").WithDefault(true),
					cmds.BoolOption("boolF", "a bool").WithDefault(false),

					cmds.UintOption("uint", "u", "a uint").WithDefault(uint(1)),
					cmds.Uint64Option("uint64", "v", "a uint64").WithDefault(uint64(1)),
					cmds.IntOption("int", "i", "an int").WithDefault(int(1)),
					cmds.Int64Option("int64", "j", "an int64").WithDefault(int64(1)),
					cmds.FloatOption("float", "f", "a float64").WithDefault(float64(1)),
				},
			},
		},
	}

	test := func(args string, expectedOpts kvs, expectedWords words) {
		testOptionHelper(t, cmd, args, expectedOpts, expectedWords, false)
	}

	test("defaults", kvs{
		"string":    "foo",
		"strings1":  []string{"foo"},
		"strings2":  []string{"foo", "bar"},
		"dstrings1": []string{"foo"},
		"dstrings2": []string{"foo", "bar"},
		"boolT":     true,
		"boolF":     false,
		"uint":      uint(1),
		"uint64":    uint64(1),
		"int":       int(1),
		"int64":     int64(1),
		"float":     float64(1),
	}, words{})
	test("defaults --string baz --strings1=baz -b baz -b=foo -c=foo -d=foo,baz,bing -d=zip,zap -d=zorp -t=false --boolF -u=0 -v=10 -i=-5 -j=10 -f=-3.14", kvs{
		"string":    "baz",
		"strings1":  []string{"baz"},
		"strings2":  []string{"baz", "foo"},
		"dstrings1": []string{"foo"},
		"dstrings2": []string{"foo", "baz", "bing", "zip", "zap", "zorp"},
		"boolT":     false,
		"boolF":     true,
		"uint":      uint(0),
		"uint64":    uint64(10),
		"int":       int(-5),
		"int64":     int64(10),
		"float":     float64(-3.14),
	}, words{})
}

func TestArgumentParsing(t *testing.T) {
	rootCmd := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"noarg": {},
			"onearg": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
				},
			},
			"twoargs": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", true, false, "another arg"),
				},
			},
			"variadic": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, true, "some arg"),
				},
			},
			"optional": {
				Arguments: []cmds.Argument{
					cmds.StringArg("b", false, true, "another arg"),
				},
			},
			"optionalsecond": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", false, false, "another arg"),
				},
			},
			"reversedoptional": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", false, false, "some arg"),
					cmds.StringArg("b", true, false, "another arg"),
				},
			},
		},
	}

	test := func(cmd words, f *os.File, res words) {
		if f != nil {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
		}
		ctx := context.Background()
		req, err := Parse(ctx, cmd, f, rootCmd)
		if err != nil {
			t.Errorf("Command '%v' should have passed parsing: %v", cmd, err)
		}
		if !sameWords(req.Arguments, res) {
			t.Errorf("Arguments parsed from '%v' are '%v' instead of '%v'", cmd, req.Arguments, res)
		}
	}

	testFail := func(cmd words, fi *os.File, msg string) {
		_, err := Parse(context.Background(), cmd, nil, rootCmd)
		if err == nil {
			t.Errorf("Should have failed: %v", msg)
		}
	}

	test([]string{"noarg"}, nil, []string{})
	testFail([]string{"noarg", "value!"}, nil, "provided an arg, but command didn't define any")

	test([]string{"onearg", "value!"}, nil, []string{"value!"})
	testFail([]string{"onearg"}, nil, "didn't provide any args, arg is required")

	test([]string{"twoargs", "value1", "value2"}, nil, []string{"value1", "value2"})
	testFail([]string{"twoargs", "value!"}, nil, "only provided 1 arg, needs 2")
	testFail([]string{"twoargs"}, nil, "didn't provide any args, 2 required")

	test([]string{"variadic", "value!"}, nil, []string{"value!"})
	test([]string{"variadic", "value1", "value2", "value3"}, nil, []string{"value1", "value2", "value3"})
	testFail([]string{"variadic"}, nil, "didn't provide any args, 1 required")

	test([]string{"optional", "value!"}, nil, []string{"value!"})
	test([]string{"optional"}, nil, []string{})
	test([]string{"optional", "value1", "value2"}, nil, []string{"value1", "value2"})

	test([]string{"optionalsecond", "value!"}, nil, []string{"value!"})
	test([]string{"optionalsecond", "value1", "value2"}, nil, []string{"value1", "value2"})
	testFail([]string{"optionalsecond"}, nil, "didn't provide any args, 1 required")
	testFail([]string{"optionalsecond", "value1", "value2", "value3"}, nil, "provided too many args, takes 2 maximum")

	test([]string{"reversedoptional", "value1", "value2"}, nil, []string{"value1", "value2"})
	test([]string{"reversedoptional", "value!"}, nil, []string{"value!"})

	testFail([]string{"reversedoptional"}, nil, "didn't provide any args, 1 required")
	testFail([]string{"reversedoptional", "value1", "value2", "value3"}, nil, "provided too many args, only takes 1")

}

func errEq(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	return err1.Error() == err2.Error()
}

func TestBodyArgs(t *testing.T) {
	rootCmd := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"noarg": {},
			"stdinenabled": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, true, "some arg").EnableStdin(),
				},
			},
			"stdinenabled2args": &cmds.Command{
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", true, true, "another arg").EnableStdin(),
				},
			},
			"stdinenablednotvariadic": &cmds.Command{
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg").EnableStdin(),
				},
			},
			"stdinenablednotvariadic2args": &cmds.Command{
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", true, false, "another arg").EnableStdin(),
				},
			},
			"optionalsecond": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", false, false, "another arg"),
				},
			},
			"optionalstdin": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", false, false, "another arg").EnableStdin(),
				},
			},
			"optionalvariadicstdin": {
				Arguments: []cmds.Argument{
					cmds.StringArg("a", true, false, "some arg"),
					cmds.StringArg("b", false, true, "another arg").EnableStdin(),
				},
			},
		},
	}

	// Use a temp file to simulate stdin
	fileToSimulateStdin := func(t *testing.T, content string) *os.File {
		fstdin, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(fstdin.Name())

		if _, err := io.WriteString(fstdin, content); err != nil {
			t.Fatal(err)
		}
		return fstdin
	}

	fstdin1 := fileToSimulateStdin(t, "stdin1")
	fstdin12 := fileToSimulateStdin(t, "stdin1\nstdin2")
	fstdin123 := fileToSimulateStdin(t, "stdin1\nstdin2\nstdin3")

	var tcs = []struct {
		cmd              words
		f                *os.File
		posArgs, varArgs words
		parseErr         error
		bodyArgs         bool
	}{
		{
			cmd: words{"stdinenabled", "value1", "value2"}, f: nil,
			posArgs: words{"value1", "value2"}, varArgs: nil,
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled"}, f: fstdin1,
			posArgs: words{"stdin1"}, varArgs: words{},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenabled", "value1"}, f: fstdin1,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled", "value1", "value2"}, f: fstdin1,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled"}, f: fstdin12,
			posArgs: words{"stdin1"}, varArgs: words{"stdin2"},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenabled"}, f: fstdin123,
			posArgs: words{"stdin1"}, varArgs: words{"stdin2", "stdin3"},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenabled2args", "value1", "value2"}, f: nil,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled2args", "value1"}, f: fstdin1,
			posArgs: words{"value1", "stdin1"}, varArgs: words{},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenabled2args", "value1", "value2"}, f: fstdin1,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled2args", "value1", "value2", "value3"}, f: fstdin1,
			posArgs: words{"value1", "value2", "value3"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenabled2args", "value1"}, f: fstdin12,
			posArgs: words{"value1", "stdin1"}, varArgs: words{"stdin2"},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenablednotvariadic", "value1"}, f: nil,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenablednotvariadic"}, f: fstdin1,
			posArgs: words{"stdin1"}, varArgs: words{},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenablednotvariadic", "value1"}, f: fstdin1,
			posArgs: words{"value1"}, varArgs: words{"value1"},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenablednotvariadic2args", "value1", "value2"}, f: nil,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenablednotvariadic2args", "value1"}, f: fstdin1,
			posArgs: words{"value1", "stdin1"}, varArgs: words{},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"stdinenablednotvariadic2args", "value1", "value2"}, f: fstdin1,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"stdinenablednotvariadic2args"}, f: fstdin1,
			posArgs: words{}, varArgs: words{},
			parseErr: fmt.Errorf(`argument %q is required`, "a"), bodyArgs: true,
		},
		{
			cmd: words{"stdinenablednotvariadic2args", "value1"}, f: nil,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: fmt.Errorf(`argument %q is required`, "b"), bodyArgs: true,
		},
		{
			cmd: words{"noarg"}, f: fstdin1,
			posArgs: words{}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"optionalsecond", "value1", "value2"}, f: fstdin1,
			posArgs: words{"value1", "value2"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"optionalstdin", "value1"}, f: fstdin1,
			posArgs: words{"value1"}, varArgs: words{"stdin1"},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"optionalstdin", "value1"}, f: nil,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"optionalstdin"}, f: fstdin1,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: fmt.Errorf(`argument %q is required`, "a"), bodyArgs: false,
		},
		{
			cmd: words{"optionalvariadicstdin", "value1"}, f: nil,
			posArgs: words{"value1"}, varArgs: words{},
			parseErr: nil, bodyArgs: false,
		},
		{
			cmd: words{"optionalvariadicstdin", "value1"}, f: fstdin1,
			posArgs: words{"value1"}, varArgs: words{"stdin1"},
			parseErr: nil, bodyArgs: true,
		},
		{
			cmd: words{"optionalvariadicstdin", "value1"}, f: fstdin12,
			posArgs: words{"value1"}, varArgs: words{"stdin1", "stdin2"},
			parseErr: nil, bodyArgs: true,
		},
	}

	for _, tc := range tcs {
		if tc.f != nil {
			if _, err := tc.f.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
		}

		req, err := Parse(context.Background(), tc.cmd, tc.f, rootCmd)
		if err == nil {
			err = req.Command.CheckArguments(req)
		}
		if !errEq(err, tc.parseErr) {
			t.Fatalf("parsing request for cmd %q: expected error %q, got %q", tc.cmd, tc.parseErr, err)
		}
		if err != nil {
			continue
		}

		if !sameWords(req.Arguments, tc.posArgs) {
			t.Errorf("Arguments parsed from %v are %v instead of %v", tc.cmd, req.Arguments, tc.posArgs)
		}

		s := req.BodyArgs()
		if !tc.bodyArgs {
			if s != nil {
				t.Fatalf("expected no BodyArgs for cmd %q", tc.cmd)
			}
			continue
		}
		if s == nil {
			t.Fatalf("expected BodyArgs for cmd %q", tc.cmd)
		}

		var bodyArgs words
		for s.Scan() {
			bodyArgs = append(bodyArgs, s.Argument())
		}
		if err := s.Err(); err != nil {
			t.Fatal(err)
		}

		if !sameWords(bodyArgs, tc.varArgs) {
			t.Errorf("BodyArgs parsed from %v are %v instead of %v", tc.cmd, bodyArgs, tc.varArgs)
		}
	}
}

func Test_isURL(t *testing.T) {
	for _, u := range []string{
		"http://www.example.com",
		"https://www.example.com",
	} {
		if isURL(u) == nil {
			t.Errorf("expected url: %s", u)
		}
	}

	for _, u := range []string{
		"adir/afile",
		"http:/ /afile",
		"http:/a/file",
	} {
		if isURL(u) != nil {
			t.Errorf("expected non-url: %s", u)
		}
	}
}

func Test_urlBase(t *testing.T) {
	for _, test := range []struct{ url, base string }{
		{"http://host", "host"},
		{"http://host/test", "test"},
		{"http://host/test?param=val", "test"},
		{"http://host/test?param=val&param2=val", "test"},
	} {
		u, err := url.Parse(test.url)
		if err != nil {
			t.Errorf("failed to parse %q: %v", test.url, err)
			continue
		}
		if got := urlBase(u); got != test.base {
			t.Errorf("expected %q but got %q", test.base, got)
		}
	}
}

func TestFileArgs(t *testing.T) {
	rootCmd := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"fileOp": {
				Arguments: []cmds.Argument{
					cmds.FileArg("path", true, true, "The path to the file to be operated upon.").EnableRecursive().EnableStdin(),
				},
				Options: []cmds.Option{
					cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
					cmds.OptionHidden,
					cmds.OptionIgnoreRules,
					cmds.OptionIgnore,
				},
			},
		},
	}
	mkTempFile := func(t *testing.T, dir, pattern, content string) *os.File {
		pat := "test_tmpFile_"
		if pattern != "" {
			pat = pattern
		}
		tmpFile, err := ioutil.TempFile(dir, pat)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.WriteString(tmpFile, content); err != nil {
			t.Fatal(err)
		}
		return tmpFile
	}
	tmpDir1, err := ioutil.TempDir("", "parsetest_fileargs_tmpdir_")
	if err != nil {
		t.Fatal(err)
	}
	tmpDir2, err := ioutil.TempDir("", "parsetest_utildir_")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile1 := mkTempFile(t, "", "", "test1")
	tmpFile2 := mkTempFile(t, tmpDir1, "", "toBeIgnored")
	tmpFile3 := mkTempFile(t, tmpDir1, "", "test3")
	ignoreFile := mkTempFile(t, tmpDir2, "", path.Base(tmpFile2.Name()))
	tmpHiddenFile := mkTempFile(t, tmpDir1, ".test_hidden_file_*", "test")
	defer func() {
		for _, f := range []string{
			tmpDir1,
			tmpFile1.Name(),
			tmpFile2.Name(),
			tmpHiddenFile.Name(),
			tmpFile3.Name(),
			ignoreFile.Name(),
			tmpDir2,
		} {
			os.Remove(f)
		}
	}()
	var testCases = []struct {
		cmd      words
		f        *os.File
		args     words
		parseErr error
	}{
		{
			cmd:      words{"fileOp"},
			args:     nil,
			parseErr: fmt.Errorf("argument %q is required", "path"),
		},
		{
			cmd: words{"fileOp", "--ignore", path.Base(tmpFile2.Name()), tmpDir1, tmpFile1.Name()}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name()},
			parseErr: fmt.Errorf(notRecursiveFmtStr, tmpDir1, "r"),
		},
		{
			cmd: words{"fileOp", tmpFile1.Name(), "--ignore", path.Base(tmpFile2.Name()), "--ignore"}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name()},
			parseErr: fmt.Errorf("missing argument for option %q", "ignore"),
		},
		{
			cmd: words{"fileOp", "-r", "--ignore", path.Base(tmpFile2.Name()), tmpDir1, tmpFile1.Name()}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name()},
			parseErr: nil,
		},
		{
			cmd: words{"fileOp", "--hidden", "-r", "--ignore", path.Base(tmpFile2.Name()), tmpDir1, tmpFile1.Name()}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name(), tmpHiddenFile.Name()},
			parseErr: nil,
		},
		{
			cmd: words{"fileOp", "-r", "--ignore", path.Base(tmpFile2.Name()), tmpDir1, tmpFile1.Name(), "--ignore", "anotherRule"}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name()},
			parseErr: nil,
		},
		{
			cmd: words{"fileOp", "-r", "--ignore-rules-path", ignoreFile.Name(), tmpDir1, tmpFile1.Name()}, f: nil,
			args:     words{tmpDir1, tmpFile1.Name(), tmpFile3.Name()},
			parseErr: nil,
		},
	}

	for _, tc := range testCases {
		req, err := Parse(context.Background(), tc.cmd, tc.f, rootCmd)
		if err == nil {
			err = req.Command.CheckArguments(req)
		}
		if !errEq(err, tc.parseErr) {
			t.Fatalf("parsing request for cmd %q: expected error %q, got %q", tc.cmd, tc.parseErr, err)
		}
		if err != nil {
			continue
		}

		if len(tc.args) == 0 {
			continue
		}
		expectedFileMap := make(map[string]bool)
		for _, arg := range tc.args {
			expectedFileMap[path.Base(arg)] = false
		}
		it := req.Files.Entries()
		for it.Next() {
			name := it.Name()
			if _, ok := expectedFileMap[name]; ok {
				expectedFileMap[name] = true
			} else {
				t.Errorf("found unexpected file %q in request %v", name, req)
			}
			file := it.Node()
			files.Walk(file, func(fpath string, nd files.Node) error {
				if fpath != "" {
					if _, ok := expectedFileMap[fpath]; ok {
						expectedFileMap[fpath] = true
					} else {
						t.Errorf("found unexpected file %q in request file arguments", fpath)
					}
				}
				return nil
			})
		}
		for p, found := range expectedFileMap {
			if !found {
				t.Errorf("failed to find expected path %q in req %v", p, req)
			}
		}
	}
}
