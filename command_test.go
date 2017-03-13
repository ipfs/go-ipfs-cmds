package cmds

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-cmds/cmdsutil"
)

type dummyCloser struct{}

func (c dummyCloser) Close() error {
	return nil
}

func newBufferResponseEmitter() ResponseEmitter {
	buf := bytes.NewBuffer(nil)
	wc := writecloser{Writer: buf}
	return NewWriterResponseEmitter(wc, nil, Encoders[Text])
}

func noop(req Request, re ResponseEmitter) {
	return
}

type writecloser struct {
	io.Writer
	io.Closer
}

func TestOptionValidation(t *testing.T) {
	cmd := Command{
		Options: []cmdsutil.Option{
			cmdsutil.IntOption("b", "beep", "enables beeper"),
			cmdsutil.StringOption("B", "boop", "password for booper"),
		},
		Run: noop,
	}

	opts, _ := cmd.GetOptions(nil)

	re := newBufferResponseEmitter()
	req, _ := NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("beep", true)
	err := cmd.Call(req, re)
	if err == nil {
		t.Error("Should have failed (incorrect type)")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("beep", 5)
	err = cmd.Call(req, re)
	if err != nil {
		t.Error(err, "Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("beep", 5)
	req.SetOption("boop", "test")
	err = cmd.Call(req, re)
	if err != nil {
		t.Error("Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("b", 5)
	req.SetOption("B", "test")
	err = cmd.Call(req, re)
	if err != nil {
		t.Error("Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("foo", 5)
	err = cmd.Call(req, re)
	if err != nil {
		t.Error("Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption(cmdsutil.EncShort, "json")
	err = cmd.Call(req, re)
	if err != nil {
		t.Error("Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, nil, opts)
	req.SetOption("b", "100")
	err = cmd.Call(req, re)
	if err != nil {
		t.Error("Should have passed")
	}

	re = newBufferResponseEmitter()
	req, _ = NewRequest(nil, nil, nil, nil, &cmd, opts)
	req.SetOption("b", ":)")
	err = cmd.Call(req, re)
	if err == nil {
		t.Error("Should have failed (string value not convertible to int)")
	}

	err = req.SetOptions(map[string]interface{}{
		"b": 100,
	})
	if err != nil {
		t.Error("Should have passed")
	}

	err = req.SetOptions(map[string]interface{}{
		"b": ":)",
	})
	if err == nil {
		t.Error("Should have failed (string value not convertible to int)")
	}
}

func TestRegistration(t *testing.T) {
	cmdA := &Command{
		Options: []cmdsutil.Option{
			cmdsutil.IntOption("beep", "number of beeps"),
		},
		Run: noop,
	}

	cmdB := &Command{
		Options: []cmdsutil.Option{
			cmdsutil.IntOption("beep", "number of beeps"),
		},
		Run: noop,
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmdC := &Command{
		Options: []cmdsutil.Option{
			cmdsutil.StringOption("encoding", "data encoding type"),
		},
		Run: noop,
	}

	path := []string{"a"}
	_, err := cmdB.GetOptions(path)
	if err == nil {
		t.Error("Should have failed (option name collision)")
	}

	_, err = cmdC.GetOptions(nil)
	if err == nil {
		t.Error("Should have failed (option name collision with global options)")
	}
}

func TestResolving(t *testing.T) {
	cmdC := &Command{}
	cmdB := &Command{
		Subcommands: map[string]*Command{
			"c": cmdC,
		},
	}
	cmdB2 := &Command{}
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": cmdB,
			"B": cmdB2,
		},
	}
	cmd := &Command{
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmds, err := cmd.Resolve([]string{"a", "b", "c"})
	if err != nil {
		t.Error(err)
	}
	if len(cmds) != 4 || cmds[0] != cmd || cmds[1] != cmdA || cmds[2] != cmdB || cmds[3] != cmdC {
		t.Error("Returned command path is different than expected", cmds)
	}
}

func TestWalking(t *testing.T) {
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": &Command{},
			"B": &Command{},
		},
	}
	i := 0
	cmdA.Walk(func(c *Command) {
		i = i + 1
	})
	if i != 3 {
		t.Error("Command tree walk didn't work, expected 3 got:", i)
	}
}

func TestHelpProcessing(t *testing.T) {
	cmdB := &Command{
		Helptext: cmdsutil.HelpText{
			ShortDescription: "This is other short",
		},
	}
	cmdA := &Command{
		Helptext: cmdsutil.HelpText{
			ShortDescription: "This is short",
		},
		Subcommands: map[string]*Command{
			"a": cmdB,
		},
	}
	cmdA.ProcessHelp()
	if len(cmdA.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
	if len(cmdB.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
}

type postRunTestCase struct {
	length      uint64
	err         *cmdsutil.Error
	emit        []interface{}
	postRun     func(Request, Response) Response
	next        []interface{}
	finalLength uint64
}

func TestPostRun(t *testing.T) {
	var testcases = []postRunTestCase{
		postRunTestCase{
			length:      3,
			err:         nil,
			emit:        []interface{}{7},
			finalLength: 4,
			next:        []interface{}{14},
			postRun: func(req Request, res Response) Response {
				re, res_ := NewChanResponsePair(req)

				re.SetLength(res.Length() + 1)
				go func() {
					defer re.Close()

					for {
						v, err := res.Next()
						if err == io.EOF {
							return
						}
						if err != nil {
							re.SetError(err, cmdsutil.ErrNormal)
							t.Fatal(err)
							return
						}

						i := v.(int)

						err = re.Emit(2 * i)
						if err != nil {
							re.SetError(err, cmdsutil.ErrNormal)
							return
						}
					}
				}()
				return res_
			},
		},
	}

	for _, tc := range testcases {
		cmd := &Command{
			Run: func(req Request, re ResponseEmitter) {
				re.SetLength(tc.length)

				go func() {
					for _, v := range tc.emit {
						re.Emit(v)
					}
					err := re.Close()
					if err != nil {
						t.Fatal(err)
					}
				}()
			},
			PostRun: map[EncodingType]func(req Request, res Response) Response{
				CLI: tc.postRun,
			},
		}

		cmdOpts, _ := cmd.GetOptions(nil)

		req, _ := NewRequest(nil, nil, nil, nil, nil, cmdOpts)
		req.SetOption(cmdsutil.EncShort, CLI)

		re, res := NewChanResponsePair(req)

		err := cmd.Call(req, re)
		if err != nil {
			t.Fatal(err)
		}

		opts := req.Options()
		if opts == nil {
			t.Fatal("req.Options() is nil")
		}

		encTypeIface := opts[cmdsutil.EncShort]
		if encTypeIface == nil {
			t.Fatal("req.Options()[cmdsutil.EncShort] is nil")
		}

		encType := EncodingType(encTypeIface.(string))
		if encType == "" {
			t.Fatal("no encoding type")
		}

		if encType != CLI {
			t.Fatal("wrong encoding type")
		}

		res = cmd.PostRun[encType](req, res)

		l := res.Length()
		if l != tc.finalLength {
			t.Fatal("wrong final length")
		}

		for _, x := range tc.next {
			ch := make(chan interface{})

			go func() {
				v, err := res.Next()
				if err != nil {
					close(ch)
					t.Fatal(err)
				}

				ch <- v
			}()

			select {
			case v, ok := <-ch:
				if !ok {
					t.Fatal("error checking all next values - channel closed")
				}
				if x != v {
					t.Fatalf("final check of emitted values failed. got %v but expected %v", v, x)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatal("too few values in next")
			}
		}
	}
}
