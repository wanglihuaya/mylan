//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// stopScan puts the scan in its own process group and kills that group when
// the command is cancelled, so a timed-out ping does not keep running.
func stopScan(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
