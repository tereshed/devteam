package sandbox

import "io"

// Эти обёртки нужны только для интеграционных тестов из других пакетов
// (Sprint 15.36 — internal/service/agent_bypass_permissions_e2e_test.go).
// Они экспонируют файл-приватные функции, не расширяя публичный API SandboxRunner.

// ExportMergeSandboxEnvForTesting — внешняя обёртка над mergeSandboxEnv.
func ExportMergeSandboxEnvForTesting(opts SandboxOptions) []string {
	return mergeSandboxEnv(opts)
}

// ExportBuildPromptContextTarForTesting — внешняя обёртка над buildPromptContextTar.
func ExportBuildPromptContextTarForTesting(instruction, contextText string, settings *AgentSettingsBundle) (io.ReadCloser, error) {
	return buildPromptContextTar(instruction, contextText, settings)
}
