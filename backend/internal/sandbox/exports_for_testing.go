//go:build test_export

package sandbox

import "io"

// Эти обёртки нужны только для интеграционных тестов из других пакетов
// (Sprint 15.36 — internal/service/agent_bypass_permissions_e2e_test.go).
// Build-tag test_export исключает их из production-бинаря; тесты, использующие
// эти символы, ОБЯЗАНЫ объявлять `//go:build test_export` в первой строке.

// ExportMergeSandboxEnvForTesting — внешняя обёртка над mergeSandboxEnv.
func ExportMergeSandboxEnvForTesting(opts SandboxOptions) []string {
	return mergeSandboxEnv(opts)
}

// ExportBuildPromptContextTarForTesting — внешняя обёртка над buildPromptContextTar.
func ExportBuildPromptContextTarForTesting(instruction, contextText string, settings *AgentSettingsBundle) (io.ReadCloser, error) {
	return buildPromptContextTar(instruction, contextText, settings)
}
