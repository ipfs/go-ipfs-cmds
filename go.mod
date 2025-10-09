module github.com/ipfs/go-ipfs-cmds

go 1.24.0

require (
	github.com/ipfs/boxo v0.34.0
	github.com/ipfs/go-log/v2 v2.8.2
	github.com/rs/cors v1.11.1
	github.com/texttheater/golang-levenshtein v1.0.1
	golang.org/x/term v0.34.0
)

require (
	github.com/crackcomm/go-gitignore v0.0.0-20241020182519-7843d2ba8fdf // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

retract v1.0.22 // old gx tag accidentally pushed as go tag

retract v2.0.1+incompatible // old gx tag

retract v2.0.2+incompatible // we need to use a newer version than v2.0.1 to retract v2.0.1+incompatible, but we can retract ourself directly once done
