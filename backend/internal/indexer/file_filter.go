package indexer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxFileSize     = 5 * 1024 * 1024 // 5MB
	FastFilterSize  = 512             // 512 bytes for MIME check
)

// FileFilter предоставляет методы для фильтрации и валидации файлов
type FileFilter struct {
	baseDir string
}

func NewFileFilter(baseDir string) *FileFilter {
	return &FileFilter{baseDir: filepath.Clean(baseDir)}
}

// ValidatePath проверяет путь на Path Traversal и нормализует его
func (f *FileFilter) ValidatePath(path string) (string, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(f.baseDir, path)
	}
	
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	rel, err := filepath.Rel(f.baseDir, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}

	// Нормализуем разделители к /
	return filepath.ToSlash(rel), nil
}

// ShouldProcess проверяет, нужно ли обрабатывать файл (размер, MIME-тип)
func (f *FileFilter) ShouldProcess(absPath string) (bool, error) {
	// 1. Используем Lstat для проверки симлинков
	lstatInfo, err := os.Lstat(absPath)
	if err != nil {
		return false, err
	}

	// Игнорируем симлинки, директории и пустые файлы
	if lstatInfo.Mode()&os.ModeSymlink != 0 || lstatInfo.IsDir() || lstatInfo.Size() == 0 {
		return false, nil
	}

	// Hard Limit: 5MB
	if lstatInfo.Size() > MaxFileSize {
		return false, nil
	}

	// 2. Открываем файл для защиты от TOCTOU
	file, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// 3. Проверяем, что файл не был подменен (os.SameFile)
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !os.SameFile(lstatInfo, info) {
		return false, nil // Файл подменили между Lstat и Open
	}

	// Fast Filtering: MIME-тип на первых 512 байтах
	buffer := make([]byte, FastFilterSize)
	n, err := io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}

	mimeType := http.DetectContentType(buffer[:n])
	
	// Разрешаем текстовые файлы и некоторые специфичные типы
	if !strings.HasPrefix(mimeType, "text/") && 
	   mimeType != "application/octet-stream" && 
	   mimeType != "application/x-javascript" &&
	   mimeType != "application/json" {
		return false, nil
	}

	// Дополнительная эвристика на бинарные данные (наличие нулевых байтов)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return false, nil
		}
	}

	return true, nil
}
