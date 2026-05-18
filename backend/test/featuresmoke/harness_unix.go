//go:build featuresmoke && !windows

package featuresmoke

import (
	"os/exec"
	"syscall"
)

// configureSysProcAttr — на Unix запускаем процесс в новой process-group
// (Setpgid=true). Это нужно, чтобы лог процесса не получал сигналы родительского
// тестового runner'а при Ctrl-C, и чтобы при желании можно было сигналить всю
// группу (мы намеренно так не делаем — см. killTree). Кроссплатформенно эта
// настройка не выражается, поэтому live в отдельном файле с build-тагом.
func configureSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
