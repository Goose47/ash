//go:build windows

package terminal

import (
	"golang.org/x/sys/windows"
	"os"
)

func GetTerminalSize() (width, height int, err error) {
	handle := windows.Handle(os.Stdout.Fd())
	var info windows.ConsoleScreenBufferInfo
	err = windows.GetConsoleScreenBufferInfo(handle, &info)
	if err != nil {
		return 0, 0, err
	}
	width = int(info.Window.Right - info.Window.Left + 1)
	height = int(info.Window.Bottom - info.Window.Top + 1)
	return
}
