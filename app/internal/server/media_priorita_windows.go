package server

import (
	"os/exec"
	"syscall"
)

// prioritaBassa: ffmpeg lavora sotto la priorità normale e senza finestra, così Gaster Bless e il PC restano reattivi.
func prioritaBassa(cmd *exec.Cmd) {
	const belowNormal, senzaFinestra = 0x00004000, 0x08000000 // BELOW_NORMAL_PRIORITY_CLASS, CREATE_NO_WINDOW
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: belowNormal | senzaFinestra}
}
