package resource

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: the process gets no console of its own.
const createNoWindow = 0x08000000

// Hide keeps the console of a program the app runs out of sight: the engine, tomoru-asr and
// llama-server are console programs, and Windows would open a window for each.
func Hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
