//go:build windows

package cmdutil

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func PrepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
