package gitprovider

import (
	"fmt"
	"strings"
)

func validateGitRefName(ref string) error {
	r := strings.TrimSpace(ref)
	if r == "" {
		return fmt.Errorf("gitprovider: empty ref")
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
	if strings.Contains(s, "..") {
		return ErrUnsafePath
	}
	return nil
}

// validateGitBranchForClone отклоняет значения, которые git может принять за флаги после -b/--branch.
func validateGitBranchForClone(branch string) error {
	if strings.TrimSpace(branch) == "" {
		return nil
	}
	return validateNonFlagGitString(branch)
}
