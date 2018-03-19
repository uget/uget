package console

import (
	"os"

	"golang.org/x/sys/unix"
)

// GetWinSize queries the window size of the terminal emulator
func GetWinSize() (uint16, uint16, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	return ws.Row, ws.Col, err
}
