package cli

import "syscall"

func isErrnoNotSupported(err error) bool {
	switch err {
	case
		// Operation not supported
		syscall.EINVAL, syscall.EPLAN9,
		// Sync on os.Stdin or os.Stderr returns "permission denied".
		syscall.EPERM:
		return true
	}
	return false
}
