package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// PatchAgentRequest — тело PATCH агента: три-состояние omit / JSON null / значение.
// encoding/json с *T не различает omit и null — разбор через map[string]json.RawMessage.
type PatchAgentRequest struct {
	modelSet, modelNull bool
	modelVal            *string

	promptIDSet, promptIDNull bool
	promptIDVal               *uuid.UUID

	codeBackendSet, codeBackendNull bool
	codeBackendVal                  *string

	// provider_kind — Sprint 15.e2e: per-agent LLM-провайдер (kind в `agents.provider_kind`).
	providerKindSet, providerKindNull bool
	providerKindVal                   *string

	isActiveSet bool
	isActiveVal bool

	toolBindingsSet bool
	toolBindingsIDs []uuid.UUID // raw order from JSON; dedup в сервисе
}

// UnmarshalJSON реализует различие отсутствующего ключа и explicit null.
func (p *PatchAgentRequest) UnmarshalJSON(data []byte) error {
	*p = PatchAgentRequest{}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["model"]; ok {
		p.modelSet = true
		if isJSONNull(v) {
			p.modelNull = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			p.modelVal = &s
		}
	}
	if v, ok := raw["prompt_id"]; ok {
		p.promptIDSet = true
		if isJSONNull(v) {
			p.promptIDNull = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return err
			}
			p.promptIDVal = &id
		}
	}
	if v, ok := raw["code_backend"]; ok {
		p.codeBackendSet = true
		if isJSONNull(v) {
			p.codeBackendNull = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			p.codeBackendVal = &s
		}
	}
	if v, ok := raw["provider_kind"]; ok {
		p.providerKindSet = true
		if isJSONNull(v) {
			p.providerKindNull = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			p.providerKindVal = &s
		}
	}
	if v, ok := raw["is_active"]; ok {
		p.isActiveSet = true
		if isJSONNull(v) {
			return errors.New("is_active cannot be null")
		}
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return err
		}
		p.isActiveVal = b
	}
	if v, ok := raw["tool_bindings"]; ok {
		p.toolBindingsSet = true
		if isJSONNull(v) {
			return errors.New("tool_bindings cannot be null")
		}
		var elems []json.RawMessage
		if err := json.Unmarshal(v, &elems); err != nil {
			return err
		}
		for _, elem := range elems {
			var obj struct {
				ToolDefinitionID string `json:"tool_definition_id"`
			}
			if err := json.Unmarshal(elem, &obj); err != nil {
				return err
			}
			id, err := uuid.Parse(strings.TrimSpace(obj.ToolDefinitionID))
			if err != nil {
				return err
			}
			p.toolBindingsIDs = append(p.toolBindingsIDs, id)
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// ModelPresent true если ключ "model" был в JSON.
func (p PatchAgentRequest) ModelPresent() bool { return p.modelSet }

// ModelClear true если явный null (сброс в БД).
func (p PatchAgentRequest) ModelClear() bool { return p.modelSet && p.modelNull }

// ModelValue возвращает значение, если ключ был и это не null.
func (p PatchAgentRequest) ModelValue() (string, bool) {
	if !p.modelSet || p.modelNull {
		return "", false
	}
	if p.modelVal == nil {
		return "", false
	}
	return *p.modelVal, true
}

func (p PatchAgentRequest) PromptIDPresent() bool { return p.promptIDSet }

func (p PatchAgentRequest) PromptIDClear() bool { return p.promptIDSet && p.promptIDNull }

func (p PatchAgentRequest) PromptIDValue() (uuid.UUID, bool) {
	if !p.promptIDSet || p.promptIDNull || p.promptIDVal == nil {
		return uuid.Nil, false
	}
	return *p.promptIDVal, true
}

func (p PatchAgentRequest) CodeBackendPresent() bool { return p.codeBackendSet }

func (p PatchAgentRequest) CodeBackendClear() bool { return p.codeBackendSet && p.codeBackendNull }

func (p PatchAgentRequest) CodeBackendValue() (string, bool) {
	if !p.codeBackendSet || p.codeBackendNull {
		return "", false
	}
	if p.codeBackendVal == nil {
		return "", false
	}
	return *p.codeBackendVal, true
}

// ProviderKindPresent true если ключ "provider_kind" был в JSON (Sprint 15.e2e).
func (p PatchAgentRequest) ProviderKindPresent() bool { return p.providerKindSet }

// ProviderKindClear true если явный null (сброс kind в БД).
func (p PatchAgentRequest) ProviderKindClear() bool {
	return p.providerKindSet && p.providerKindNull
}

// ProviderKindValue возвращает значение kind, если ключ был и это не null.
func (p PatchAgentRequest) ProviderKindValue() (string, bool) {
	if !p.providerKindSet || p.providerKindNull {
		return "", false
	}
	if p.providerKindVal == nil {
		return "", false
	}
	return *p.providerKindVal, true
}

func (p PatchAgentRequest) IsActivePresent() bool { return p.isActiveSet }

func (p PatchAgentRequest) IsActiveValue() (bool, bool) {
	if !p.isActiveSet {
		return false, false
	}
	return p.isActiveVal, true
}

// ToolBindingsPresent true если ключ "tool_bindings" был в JSON.
func (p PatchAgentRequest) ToolBindingsPresent() bool { return p.toolBindingsSet }

// ToolBindingsRawIDs возвращает распарсенные id (возможны дубликаты; dedup в сервисе).
func (p PatchAgentRequest) ToolBindingsRawIDs() []uuid.UUID {
	if !p.toolBindingsSet {
		return nil
	}
	return p.toolBindingsIDs
}
