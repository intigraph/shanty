//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package stread

import (
	"os"
	"syscall"
)

// The kernel releases this lock even if a renderer dies. Never unlink the lock
// file: doing so would let two refreshers lock different inodes for one key.
func inboxLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
