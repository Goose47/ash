//go:build !windows

package terminal

import (
	"golang.org/x/term"
	"os"
)

func GetTerminalSize() (width, height int, err error) {
	return term.GetSize(int(os.Stdout.Fd()))
}
