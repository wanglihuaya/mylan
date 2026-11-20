//go:build windows

package main

import "os/exec"

// stopScan leaves cancellation to CommandContext. Windows has no process
// group to signal the way Unix does, so the timeout kills the scan process.
func stopScan(cmd *exec.Cmd) {}
