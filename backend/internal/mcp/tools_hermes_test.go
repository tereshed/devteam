package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
)

// Sprint 16.C — hermes_toolsets_list возвращает встроенный каталог.

func TestHermesToolsetsListHandler_RequiresAuth(t *testing.T) {
	res, _, err := hermesToolsetsListHandler(context.Background(), nil, &HermesToolsetsListParams{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected validation error result, got %+v", res)
	}
}

func TestHermesToolsetsListHandler_ReturnsCatalog(t *testing.T) {
	uid := uuid.New()
	ctx := context.WithValue(context.Background(), CtxKeyUserID, uid)
	res, data, err := hermesToolsetsListHandler(ctx, nil, &HermesToolsetsListParams{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	resp, ok := data.(*Response)
	if !ok {
		t.Fatalf("data is not *Response: %T", data)
	}
	items, ok := resp.Data.([]HermesToolsetItemDTO)
	if !ok {
		t.Fatalf("resp.Data is not []HermesToolsetItemDTO: %T", resp.Data)
	}
	if len(items) != len(service.HermesToolsetCatalog) {
		t.Fatalf("got %d items, want %d", len(items), len(service.HermesToolsetCatalog))
	}
	hasFileOps := false
	for _, it := range items {
		if it.Name == "file_ops" {
			hasFileOps = true
		}
	}
	if !hasFileOps {
		t.Fatalf("file_ops missing from catalog: %+v", items)
	}

	// Round-trip JSON to ensure DTO is JSON-serializable.
	if _, err := json.Marshal(resp); err != nil {
		t.Fatalf("json marshal: %v", err)
	}
}
