//go:build !windows

package cmdutil

import "os/exec"

func PrepareCmd(cmd *exec.Cmd) {}
