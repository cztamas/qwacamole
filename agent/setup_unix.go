//go:build linux || darwin

package agent

import (
	"os"
	"syscall"
)

func setupRootOwned(info os.FileInfo) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Uid == 0
}

func setupOwnedBy(info os.FileInfo, uid uint32) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Uid == uid
}
