//go:build featuresmoke && windows

package featuresmoke

import "os/exec"

// configureSysProcAttr — на Windows нет аналога Setpgid; ничего не делаем.
// killTree использует кроссплатформенный cmd.Process.Kill (см. harness.go).
func configureSysProcAttr(cmd *exec.Cmd) {}
