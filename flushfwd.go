package cmds

type Flusher interface {
	Flush() error
}

type flushfwder struct {
	ResponseEmitter
	Flusher
}

func (ff *flushfwder) Close() error {
	err := ff.Flush()
	if err != nil {
		return err
	}

	return ff.ResponseEmitter.Close()
}

// TODO: no documentation; [54dbca2b-17f2-42a8-af93-c8d713866138]
func NewFlushForwarder(re ResponseEmitter, f Flusher) ResponseEmitter {
	return &flushfwder{ResponseEmitter: re, Flusher: f}
}
