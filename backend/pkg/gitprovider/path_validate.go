package gitprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateGitTransportInjection(s string) error {
	low := strings.ToLower(s)
	if strings.Contains(low, "--upload-pack") || strings.Contains(low, "--receive-pack") {
		return ErrUnsafePath
	}
	return nil
}

func validateGitRefName(ref string) error {
	r := strings.TrimSpace(ref)
	if r == "" {
		return fmt.Errorf("gitprovider: empty ref")
	}
	if err := validateGitTransportInjection(r); err != nil {
		return err
	}
	if strings.Contains(r, "..") {
		return ErrUnsafePath
	}
	if strings.ContainsAny(r, "\n\r\x00") {
		return ErrUnsafePath
	}
	return nil
}

func validateGitPathForBlob(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return fmt.Errorf("gitprovider: empty file path")
	}
	if strings.Contains(p, "..") {
		return ErrUnsafePath
	}
	if strings.Contains(p, `\`) {
		return ErrUnsafePath
	}
	if strings.HasPrefix(p, "/") {
		return ErrUnsafePath
	}
	return nil
}

func validateGitPathForCommit(path string) error {
	// Элементы для git add после "--"
	if err := validateGitPathForBlob(path); err != nil {
		return err
	}
	return nil
}

func validateNonFlagGitString(s string) error {
	if strings.HasPrefix(strings.TrimSpace(s), "-") {
		return fmt.Errorf("gitprovider: value must not start with '-'")
	}
	if err := validateGitTransportInjection(s); err != nil {
		return err
	}
	if strings.Contains(s, "..") {
		return ErrUnsafePath
	}
	return nil
}

// validateCloneDestPath ограничивает целевой путь клонирования каталогами cwd или системного temp (см. задачу 4.9).
func validateCloneDestPath(dest string) error {
	d := strings.TrimSpace(dest)
	if d == "" {
		return fmt.Errorf("gitprovider: empty clone destination path")
	}
	if strings.ContainsAny(d, "\n\r\x00") {
		return ErrUnsafePath
	}
	destAbs, err := filepath.Abs(d)
	if err != nil {
		return fmt.Errorf("gitprovider: invalid clone destination: %w", err)
	}
	sep := string(filepath.Separator)
	var bases []string
	if wd, err := os.Getwd(); err == nil {
		if b, err := filepath.Abs(wd); err == nil {
			bases = append(bases, b)
		}
	}
	if tmp := filepath.Clean(os.TempDir()); tmp != "" {
		if b, err := filepath.Abs(tmp); err == nil {
			bases = append(bases, b)
		}
	}
	if len(bases) == 0 {
		return fmt.Errorf("gitprovider: cannot resolve clone destination policy base")
	}
	for _, baseAbs := range bases {
		rel, err := filepath.Rel(baseAbs, destAbs)
		if err != nil {
			continue
		}
		if rel == "." {
			return nil
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
			continue
		}
		return nil
	}
	return ErrUnsafePath
}

// validateGitBranchForClone отклоняет значения, которые git может принять за флаги после -b/--branch.
func validateGitBranchForClone(branch string) error {
	if strings.TrimSpace(branch) == "" {
		return nil
	}
	return validateNonFlagGitString(branch)
}
