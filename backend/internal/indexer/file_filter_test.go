package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileFilter_ValidatePath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "indexer-test")
	defer os.RemoveAll(tmpDir)
	
	// Получаем абсолютный путь к временной директории
	tmpDir, _ = filepath.Abs(tmpDir)

	filter := NewFileFilter(tmpDir)

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"Normal path", filepath.Join(tmpDir, "src/main.go"), "src/main.go", false},
		{"Relative path", "src/main.go", "src/main.go", false},
		{"Path traversal", "../outside.go", "", true},
		{"Path traversal absolute", "/etc/passwd", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filter.ValidatePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFileFilter_ShouldProcess(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "indexer-test")
	defer os.RemoveAll(tmpDir)

	filter := NewFileFilter(tmpDir)

	// 1. Обычный текстовый файл
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("hello world"), 0644)
	should, err := filter.ShouldProcess(txtPath)
	assert.NoError(t, err)
	assert.True(t, should)

	// 2. Слишком большой файл
	bigPath := filepath.Join(tmpDir, "big.txt")
	bigFile, _ := os.Create(bigPath)
	bigFile.Truncate(MaxFileSize + 1)
	bigFile.Close()
	should, err = filter.ShouldProcess(bigPath)
	assert.NoError(t, err)
	assert.False(t, should)

	// 3. Бинарный файл (с нулевым байтом)
	binPath := filepath.Join(tmpDir, "bin.dat")
	os.WriteFile(binPath, []byte{0x00, 0x01, 0x02}, 0644)
	should, err = filter.ShouldProcess(binPath)
	assert.NoError(t, err)
	assert.False(t, should)
}
