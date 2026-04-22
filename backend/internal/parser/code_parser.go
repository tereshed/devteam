package parser

import (
	"context"
)

// Node представляет узел в AST
type Node struct {
	Type      string
	Content   string
	StartLine int
	EndLine   int
	Symbol    string // Имя функции, метода или класса
}

// CodeParser интерфейс для разбора структуры файлов
type CodeParser interface {
	// Parse разбивает файл на логические блоки (функции, методы и т.д.)
	Parse(ctx context.Context, language string, content []byte) ([]Node, error)
	
	// GetLanguageByExtension возвращает язык по расширению файла
	GetLanguageByExtension(ext string) string

	// Reset сбрасывает состояние парсера (для использования в sync.Pool)
	Reset()
}
