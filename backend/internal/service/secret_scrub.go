package service

import "regexp"

// secret_scrub.go — переносится из удалённого result_processor.go (Sprint 17 cleanup).
// Используется orchestrator_context_builder.go для маскирования секретов в строках,
// идущих в логи / промпты.
//
// Это НЕ заменяет основную защиту через internal/logging/redact.go и
// pkg/crypto-шифрование колонок БД — это дополнительный слой scrubbing'а в текстах.

// secretPatterns — скомпилированы один раз при загрузке пакета (MustCompile).
// Каждое выражение покрывает типичный формат секрета в логах: ENV-style, GitHub PAT,
// Bearer-токены, базовые auth-форматы.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|auth[_-]?token|secret|password|passwd|bearer|token)[\s:=]+[^\s,]{8,}`),
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9\-._~+/]+=*`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)[^\s]+`),
}
