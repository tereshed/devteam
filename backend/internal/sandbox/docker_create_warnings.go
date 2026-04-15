package sandbox

import (
	"strings"
)

// isDockerSwapLimitKernelUnsupportedWarning — ядро/cgroup не поддерживают лимит swap (swapaccount и т.п.).
// Такие предупреждения не должны прерывать RunTask: память по-прежнему ограничена Memory (задача 5.9).
func isDockerSwapLimitKernelUnsupportedWarning(w string) bool {
	s := strings.ToLower(w)
	return strings.Contains(s, "kernel does not support swap limit capabilities") ||
		strings.Contains(s, "does not support swap limit capabilities") ||
		strings.Contains(s, "swap limit") && strings.Contains(s, "not supported")
}

// isDockerCreateWarningFatal — явное подмножество предупреждений ContainerCreate, после которых
// контейнер нельзя считать безопасно ограниченным. Класс swap-kernel сюда не включается.
// По умолчанию (пустой allowlist фатальных строк) — только WARN в лог (см. задачу 5.9).
func isDockerCreateWarningFatal(w string) bool {
	_ = w
	return false
}
