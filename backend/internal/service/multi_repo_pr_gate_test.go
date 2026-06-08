package service

import (
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestExtractRepoSlug(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"top-level", `{"repo_slug":"core","title":"x"}`, "core"},
		{"nested content", `{"content":{"repo_slug":"ui"}}`, "ui"},
		{"absent", `{"title":"x"}`, ""},
		{"empty slug", `{"repo_slug":""}`, ""},
		{"invalid json", `not-json`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractRepoSlug(datatypes.JSON([]byte(c.content)))
			if got != c.want {
				t.Fatalf("extractRepoSlug(%s) = %q, want %q", c.content, got, c.want)
			}
		})
	}
}

func TestResolveSlugFromArtifact_ClimbsParentChain(t *testing.T) {
	// subtask_description (repo_slug=core) ← code_diff ← review(code_diff)
	subtaskID := uuid.New()
	codeDiffID := uuid.New()
	reviewID := uuid.New()

	subtask := &models.Artifact{
		ID:      subtaskID,
		Kind:    models.ArtifactKindSubtaskDescription,
		Content: datatypes.JSON([]byte(`{"repo_slug":"core","title":"do thing"}`)),
	}
	codeDiff := &models.Artifact{
		ID:       codeDiffID,
		ParentID: &subtaskID,
		Kind:     models.ArtifactKindCodeDiff,
		Content:  datatypes.JSON([]byte(`{"diff":"...","branch_name":"task-x"}`)), // no repo_slug
	}
	review := &models.Artifact{
		ID:       reviewID,
		ParentID: &codeDiffID,
		Kind:     models.ArtifactKindReview,
		Content:  datatypes.JSON([]byte(`{"decision":"approved"}`)),
	}

	byID := map[uuid.UUID]*models.Artifact{
		subtaskID:  subtask,
		codeDiffID: codeDiff,
		reviewID:   review,
	}

	// code_diff climbs one level to subtask_description.
	if got := resolveSlugFromArtifact(codeDiff, byID); got != "core" {
		t.Fatalf("code_diff slug = %q, want core", got)
	}
	// review climbs two levels.
	if got := resolveSlugFromArtifact(review, byID); got != "core" {
		t.Fatalf("review slug = %q, want core", got)
	}
	// subtask itself.
	if got := resolveSlugFromArtifact(subtask, byID); got != "core" {
		t.Fatalf("subtask slug = %q, want core", got)
	}
}

func TestResolveSlugFromArtifact_NoSlugInChain(t *testing.T) {
	parentID := uuid.New()
	childID := uuid.New()
	parent := &models.Artifact{ID: parentID, Content: datatypes.JSON([]byte(`{"title":"x"}`))}
	child := &models.Artifact{ID: childID, ParentID: &parentID, Content: datatypes.JSON([]byte(`{"diff":"y"}`))}
	byID := map[uuid.UUID]*models.Artifact{parentID: parent, childID: child}

	if got := resolveSlugFromArtifact(child, byID); got != "" {
		t.Fatalf("slug = %q, want empty", got)
	}
}

func TestResolveSlugFromArtifact_BrokenParentRef(t *testing.T) {
	// ParentID points to an artifact not present in the map — must not panic, returns "".
	missingID := uuid.New()
	a := &models.Artifact{ID: uuid.New(), ParentID: &missingID, Content: datatypes.JSON([]byte(`{}`))}
	byID := map[uuid.UUID]*models.Artifact{a.ID: a}
	if got := resolveSlugFromArtifact(a, byID); got != "" {
		t.Fatalf("slug = %q, want empty", got)
	}
}
