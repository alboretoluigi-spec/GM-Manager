//go:build !windows

package server

import "os/exec"

// prioritaBassa: fuori da Windows (sviluppo e prove) non serve.
func prioritaBassa(cmd *exec.Cmd) {}
