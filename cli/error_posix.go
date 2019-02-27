//+build !windows

package cli

import (
	"syscall"
)

func isErrnoNotSupported(err error) bool {
	switch err {
	case syscall.EINVAL, syscall.ENOTSUP, syscall.ENOTTY:
		return true
	}
	return false
}
