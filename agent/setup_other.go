//go:build !linux && !darwin

package agent

import "os"

func setupRootOwned(os.FileInfo) bool { return false }

func setupOwnedBy(os.FileInfo, uint32) bool { return false }
