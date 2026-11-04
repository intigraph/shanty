//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package stread

import "errors"

func inboxLock(string) (func(), error) {
	return nil, errors.New("inbox refresh locking unsupported")
}
