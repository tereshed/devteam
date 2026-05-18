//go:build featuresmoke

package featuresmoke

import (
	"os"
	"testing"
)

// TestMain — централизованный cleanup для shared backend-процесса.
// go test гарантирует, что m.Run() возвращается после завершения всех тестов
// (включая t.Parallel'ные), поэтому RunGlobalCleanup безопасно вызвать здесь.
func TestMain(m *testing.M) {
	code := m.Run()
	RunGlobalCleanup()
	os.Exit(code)
}
