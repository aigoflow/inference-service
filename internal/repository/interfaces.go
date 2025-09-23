package repository

import (
	"context"

	"github.com/aigoflow/inference-service/internal/models"
)

// Repository aggregates all repository interfaces
type Repository interface {
	Grammar() GrammarRepositoryInterface
	Request() RequestRepositoryInterface
	Event() EventRepositoryInterface
}

// GrammarRepositoryInterface defines grammar storage operations
type GrammarRepositoryInterface interface {
	CreateGrammar(dir, name string, content, description string) (*models.Grammar, error)
	GetGrammar(dir, name string) (*models.Grammar, error)
	UpdateGrammar(dir, name, content, description string) (*models.Grammar, error)
	DeleteGrammar(dir, name string) error
	ListGrammars(dir string) ([]*models.Grammar, error)
	ListDirectories() ([]string, error)
	CreateDirectory(name string) error
	DeleteDirectory(name string) error
}

// RequestRepositoryInterface defines request logging operations
type RequestRepositoryInterface interface {
	LogRequest(ctx context.Context, req *models.RequestLog) error
	GetRequestLogs(ctx context.Context, limit int) ([]*models.RequestLog, error)
}

// EventRepositoryInterface defines event logging operations
type EventRepositoryInterface interface {
	LogEvent(ctx context.Context, level, code, msg string, meta map[string]interface{}) error
}