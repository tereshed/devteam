package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrPromptNotFound      = errors.New("prompt not found")
	ErrPromptAlreadyExists = errors.New("prompt with this name already exists")
)

// PromptService интерфейс бизнес-логики для промптов
type PromptService interface {
	Create(ctx context.Context, req dto.CreatePromptRequest) (*models.Prompt, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error)
	GetByName(ctx context.Context, name string) (*models.Prompt, error)
	List(ctx context.Context) ([]models.Prompt, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdatePromptRequest) (*models.Prompt, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type promptService struct {
	repo repository.PromptRepository
}

// NewPromptService создает новый сервис
func NewPromptService(repo repository.PromptRepository) PromptService {
	return &promptService{repo: repo}
}

func (s *promptService) Create(ctx context.Context, req dto.CreatePromptRequest) (*models.Prompt, error) {
	// Проверяем уникальность имени
	existing, err := s.repo.GetByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, ErrPromptAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	prompt := &models.Prompt{
		Name:        req.Name,
		Description: req.Description,
		Template:    req.Template,
		JSONSchema:  req.JSONSchema,
		IsActive:    isActive,
	}

	if err := s.repo.Create(ctx, prompt); err != nil {
		return nil, err
	}

	return prompt, nil
}

func (s *promptService) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error) {
	prompt, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPromptNotFound
		}
		return nil, err
	}
	return prompt, nil
}

func (s *promptService) GetByName(ctx context.Context, name string) (*models.Prompt, error) {
	prompt, err := s.repo.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPromptNotFound
		}
		return nil, err
	}
	return prompt, nil
}

func (s *promptService) List(ctx context.Context) ([]models.Prompt, error) {
	return s.repo.List(ctx)
}

func (s *promptService) Update(ctx context.Context, id uuid.UUID, req dto.UpdatePromptRequest) (*models.Prompt, error) {
	prompt, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPromptNotFound
		}
		return nil, err
	}

	if req.Description != "" {
		prompt.Description = req.Description
	}
	if req.Template != "" {
		prompt.Template = req.Template
	}
	if len(req.JSONSchema) > 0 {
		prompt.JSONSchema = req.JSONSchema
	}
	if req.IsActive != nil {
		prompt.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, prompt); err != nil {
		return nil, err
	}

	return prompt, nil
}

func (s *promptService) Delete(ctx context.Context, id uuid.UUID) error {
	// Проверяем существование перед удалением
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPromptNotFound
		}
		return err
	}

	return s.repo.Delete(ctx, id)
}
