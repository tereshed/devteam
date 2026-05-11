package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	llmCredMinKeyRunes = 16
	llmCredMaxKeyRunes = 4096
)

var (
	ErrLlmCredentialsConflictClearAndSet = errors.New("cannot clear and set credentials for the same provider in one request")
	ErrLlmCredentialsKeyTooShort         = errors.New("api key is too short")
	ErrLlmCredentialsKeyTooLong          = errors.New("api key is too long")
	// ErrLlmCredentialsConcurrentModify — гонка при INSERT (23505) или потерянный UPDATE; клиент может повторить PATCH.
	ErrLlmCredentialsConcurrentModify = errors.New("llm credential concurrently modified")
)

// llmPatchField — порядок провайдеров и поля PatchLlmCredentialsRequest (один раз на процесс).
var llmPatchFields = []struct {
	p   models.UserLLMProvider
	get func(*dto.PatchLlmCredentialsRequest) (set *string, clear *bool)
}{
	{models.UserLLMProviderOpenAI, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.OpenAIAPIKey, r.ClearOpenAIKey
	}},
	{models.UserLLMProviderAnthropic, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.AnthropicAPIKey, r.ClearAnthropicKey
	}},
	{models.UserLLMProviderGemini, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.GeminiAPIKey, r.ClearGeminiKey
	}},
	{models.UserLLMProviderDeepSeek, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.DeepSeekAPIKey, r.ClearDeepSeekKey
	}},
	{models.UserLLMProviderQwen, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.QwenAPIKey, r.ClearQwenKey
	}},
	{models.UserLLMProviderOpenRouter, func(r *dto.PatchLlmCredentialsRequest) (*string, *bool) {
		return r.OpenRouterAPIKey, r.ClearOpenRouterKey
	}},
}

// UserLlmCredentialService бизнес-логика пользовательских LLM-ключей.
type UserLlmCredentialService interface {
	GetMasked(ctx context.Context, userID uuid.UUID) (*dto.LlmCredentialsResponse, error)
	Patch(ctx context.Context, userID uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, userAgent string) (*dto.LlmCredentialsResponse, error)
}

type userLlmCredentialService struct {
	repo      repository.UserLlmCredentialRepository
	tx        repository.TransactionManager
	encryptor Encryptor
	log       *slog.Logger
}

func NewUserLlmCredentialService(
	repo repository.UserLlmCredentialRepository,
	tx repository.TransactionManager,
	encryptor Encryptor,
	log *slog.Logger,
) UserLlmCredentialService {
	if log == nil {
		log = slog.Default()
	}
	return &userLlmCredentialService{repo: repo, tx: tx, encryptor: encryptor, log: log}
}

func (s *userLlmCredentialService) GetMasked(ctx context.Context, userID uuid.UUID) (*dto.LlmCredentialsResponse, error) {
	rows, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	byProv := make(map[models.UserLLMProvider]models.UserLlmCredential)
	for _, row := range rows {
		byProv[row.Provider] = row
	}
	out := &dto.LlmCredentialsResponse{}
	for _, p := range models.UserLLMProvidersOrdered {
		row, ok := byProv[p]
		if !ok {
			setMaskedPreview(out, p, nil)
			continue
		}
		plain, err := s.encryptor.Decrypt(row.EncryptedKey, []byte(row.ID.String()))
		if err != nil {
			s.log.Warn("llm credential decrypt failed", "user_id", userID, "provider", string(p))
			return nil, fmt.Errorf("%w: %w", ErrDecryptionFailed, err)
		}
		m := maskAPIKey(string(plain))
		setMaskedPreview(out, p, &m)
	}
	return out, nil
}

func setMaskedPreview(out *dto.LlmCredentialsResponse, p models.UserLLMProvider, preview *string) {
	switch p {
	case models.UserLLMProviderOpenAI:
		out.OpenAI.MaskedPreview = preview
	case models.UserLLMProviderAnthropic:
		out.Anthropic.MaskedPreview = preview
	case models.UserLLMProviderGemini:
		out.Gemini.MaskedPreview = preview
	case models.UserLLMProviderDeepSeek:
		out.DeepSeek.MaskedPreview = preview
	case models.UserLLMProviderQwen:
		out.Qwen.MaskedPreview = preview
	case models.UserLLMProviderOpenRouter:
		out.OpenRouter.MaskedPreview = preview
	}
}

func maskAPIKey(plain string) string {
	rs := []rune(plain)
	if len(rs) < 4 {
		return "********"
	}
	last := string(rs[len(rs)-4:])
	return "****" + last
}

func (s *userLlmCredentialService) Patch(ctx context.Context, userID uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, userAgent string) (*dto.LlmCredentialsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("internal: nil PatchLlmCredentialsRequest")
	}

	const (
		opSet   = 1
		opClear = 2
	)

	ops := make(map[models.UserLLMProvider]int)
	newKeys := make(map[models.UserLLMProvider]string)

	for _, field := range llmPatchFields {
		setPtr, clearPtr := field.get(req)

		wantClear := clearPtr != nil && *clearPtr

		var keyStr string
		hasNonEmptyKey := false
		if setPtr != nil {
			keyStr = strings.TrimSpace(*setPtr)
			if keyStr != "" {
				hasNonEmptyKey = true
			}
		}

		if wantClear && hasNonEmptyKey {
			return nil, ErrLlmCredentialsConflictClearAndSet
		}
		if wantClear {
			ops[field.p] = opClear
			continue
		}
		if setPtr == nil {
			continue
		}
		if !hasNonEmptyKey {
			continue
		}
		nRunes := utf8.RuneCountInString(keyStr)
		if nRunes < llmCredMinKeyRunes {
			return nil, ErrLlmCredentialsKeyTooShort
		}
		if nRunes > llmCredMaxKeyRunes {
			return nil, ErrLlmCredentialsKeyTooLong
		}
		ops[field.p] = opSet
		newKeys[field.p] = keyStr
	}

	if len(ops) == 0 {
		return s.GetMasked(ctx, userID)
	}

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, field := range llmPatchFields {
			kind, ok := ops[field.p]
			if !ok {
				continue
			}
			p := field.p
			switch kind {
			case opClear:
				n, err := s.repo.DeleteByUserAndProvider(txCtx, userID, p)
				if err != nil {
					return err
				}
				if n > 0 {
					if err := s.repo.CreateAudit(txCtx, &models.UserLlmCredentialAudit{
						UserID:    userID,
						Provider:  p,
						Action:    models.UserLlmCredentialAuditClear,
						IP:        ip,
						UserAgent: userAgent,
					}); err != nil {
						return err
					}
					s.log.InfoContext(txCtx, "llm credential cleared", "user_id", userID, "provider", string(p), "result", "ok")
				}
			case opSet:
				if err := s.setProviderKey(txCtx, userID, p, newKeys[p], ip, userAgent); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetMasked(ctx, userID)
}

func (s *userLlmCredentialService) setProviderKey(txCtx context.Context, userID uuid.UUID, p models.UserLLMProvider, plain, ip, userAgent string) error {
	existing, err := s.repo.GetByUserAndProvider(txCtx, userID, p)
	if err != nil {
		return err
	}
	if existing == nil {
		return s.insertLlmCredentialOrConcurrent(txCtx, userID, p, plain, ip, userAgent)
	}
	return s.updateLlmCredentialWithRefetch(txCtx, userID, p, plain, existing, ip, userAgent)
}

func (s *userLlmCredentialService) insertLlmCredentialOrConcurrent(txCtx context.Context, userID uuid.UUID, p models.UserLLMProvider, plain, ip, userAgent string) error {
	row := &models.UserLlmCredential{
		ID:       uuid.New(),
		UserID:   userID,
		Provider: p,
	}
	enc, err := s.encryptor.Encrypt([]byte(plain), []byte(row.ID.String()))
	if err != nil {
		return err
	}
	row.EncryptedKey = enc
	err = s.repo.Create(txCtx, row)
	if err != nil {
		if repository.IsPostgresUniqueViolation(err) {
			code, cst := repository.PostgresErrorFields(err)
			s.log.WarnContext(txCtx, "llm credential set: unique violation (concurrent insert)", "user_id", userID, "provider", string(p), "pg_code", code, "constraint", cst)
			return fmt.Errorf("%w", ErrLlmCredentialsConcurrentModify)
		}
		return err
	}
	return s.auditLlmCredentialSet(txCtx, userID, p, ip, userAgent)
}

func (s *userLlmCredentialService) updateLlmCredentialWithRefetch(txCtx context.Context, userID uuid.UUID, p models.UserLLMProvider, plain string, existing *models.UserLlmCredential, ip, userAgent string) error {
	err := s.tryEncryptUpdate(txCtx, plain, existing)
	if err == nil {
		return s.auditLlmCredentialSet(txCtx, userID, p, ip, userAgent)
	}
	if !errors.Is(err, repository.ErrUserLlmCredentialNotUpdated) {
		return err
	}
	existing2, err2 := s.repo.GetByUserAndProvider(txCtx, userID, p)
	if err2 != nil {
		return err2
	}
	if existing2 == nil {
		return s.insertLlmCredentialOrConcurrent(txCtx, userID, p, plain, ip, userAgent)
	}
	err = s.tryEncryptUpdate(txCtx, plain, existing2)
	if err != nil {
		if errors.Is(err, repository.ErrUserLlmCredentialNotUpdated) {
			s.log.WarnContext(txCtx, "llm credential set: update affected 0 rows twice", "user_id", userID, "provider", string(p))
			return fmt.Errorf("%w", ErrLlmCredentialsConcurrentModify)
		}
		return err
	}
	return s.auditLlmCredentialSet(txCtx, userID, p, ip, userAgent)
}

func (s *userLlmCredentialService) tryEncryptUpdate(txCtx context.Context, plain string, row *models.UserLlmCredential) error {
	enc, err := s.encryptor.Encrypt([]byte(plain), []byte(row.ID.String()))
	if err != nil {
		return err
	}
	row.EncryptedKey = enc
	return s.repo.Update(txCtx, row)
}

func (s *userLlmCredentialService) auditLlmCredentialSet(txCtx context.Context, userID uuid.UUID, p models.UserLLMProvider, ip, userAgent string) error {
	if err := s.repo.CreateAudit(txCtx, &models.UserLlmCredentialAudit{
		UserID:    userID,
		Provider:  p,
		Action:    models.UserLlmCredentialAuditSet,
		IP:        ip,
		UserAgent: userAgent,
	}); err != nil {
		return err
	}
	s.log.InfoContext(txCtx, "llm credential set", "user_id", userID, "provider", string(p), "result", "ok")
	return nil
}
