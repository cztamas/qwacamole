//go:build windows

package qwacamole

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func lockState(path string) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); e != nil {
		f.Close()
		return nil, fmt.Errorf("binding state already owned by another process: %w", e)
	}
	return f, nil
}
