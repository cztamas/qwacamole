//go:build !darwin && !linux

package agent

import (
	"fmt"
	"os"
)

func lockState(string) (*os.File, error) {
	return nil, fmt.Errorf("local Nginx deployment is supported on Linux and macOS only")
}
