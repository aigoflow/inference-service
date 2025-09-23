package services

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aigoflow/inference-service/internal/models"
	"github.com/aigoflow/inference-service/internal/repository"
)

type GrammarService struct {
	repo *repository.GrammarRepository
}

func NewGrammarService(grammarRoot string) *GrammarService {
	return &GrammarService{
		repo: repository.NewGrammarRepository(grammarRoot),
	}
}

// CreateGrammar creates a new grammar with validation
func (s *GrammarService) CreateGrammar(req models.CreateGrammarRequest) (*models.Grammar, error) {
	// Validate grammar name
	if err := s.validateGrammarName(req.Name); err != nil {
		return nil, err
	}
	
	// Validate directory name
	if err := s.validateDirectoryName(req.Directory); err != nil {
		return nil, err
	}
	
	// Use default directory if not specified
	dir := req.Directory
	if dir == "" {
		dir = "default"
	}
	
	// Validate grammar syntax (basic check)
	if !strings.Contains(req.Grammar, "::=") {
		return nil, fmt.Errorf("invalid grammar syntax: must contain '::=' rules")
	}
	
	slog.Info("Creating grammar", "name", req.Name, "directory", dir, "size", len(req.Grammar))
	
	return s.repo.CreateGrammar(dir, req.Name, req.Grammar, req.Description)
}

// GetGrammar retrieves a specific grammar
func (s *GrammarService) GetGrammar(dir, name string) (*models.Grammar, error) {
	if dir == "" {
		dir = "default"
	}
	
	return s.repo.GetGrammar(dir, name)
}

// UpdateGrammar updates an existing grammar
func (s *GrammarService) UpdateGrammar(dir, name string, req models.UpdateGrammarRequest) (*models.Grammar, error) {
	if dir == "" {
		dir = "default"
	}
	
	// Validate grammar syntax
	if !strings.Contains(req.Grammar, "::=") {
		return nil, fmt.Errorf("invalid grammar syntax: must contain '::=' rules")
	}
	
	slog.Info("Updating grammar", "name", name, "directory", dir, "size", len(req.Grammar))
	
	return s.repo.UpdateGrammar(dir, name, req.Grammar, req.Description)
}

// DeleteGrammar removes a grammar
func (s *GrammarService) DeleteGrammar(dir, name string) error {
	if dir == "" {
		dir = "default"
	}
	
	slog.Info("Deleting grammar", "name", name, "directory", dir)
	
	return s.repo.DeleteGrammar(dir, name)
}

// ListGrammars lists all grammars in a directory
func (s *GrammarService) ListGrammars(dir string) (*models.GrammarListResponse, error) {
	if dir == "" {
		dir = "default"
	}
	
	grammars, err := s.repo.ListGrammars(dir)
	if err != nil {
		return nil, err
	}
	
	return &models.GrammarListResponse{
		Directory: dir,
		Grammars:  grammars,
	}, nil
}

// ListDirectories lists all available directories
func (s *GrammarService) ListDirectories() (*models.DirectoryListResponse, error) {
	dirs, err := s.repo.ListDirectories()
	if err != nil {
		return nil, err
	}
	
	return &models.DirectoryListResponse{
		Directories: dirs,
	}, nil
}

// CreateDirectory creates a new grammar directory
func (s *GrammarService) CreateDirectory(req models.CreateDirectoryRequest) error {
	if err := s.validateDirectoryName(req.Name); err != nil {
		return err
	}
	
	slog.Info("Creating grammar directory", "name", req.Name)
	
	return s.repo.CreateDirectory(req.Name)
}

// DeleteDirectory removes a directory and all its grammars
func (s *GrammarService) DeleteDirectory(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete default directory")
	}
	
	slog.Info("Deleting grammar directory", "name", name)
	
	return s.repo.DeleteDirectory(name)
}

// ResolveGrammar resolves a grammar reference to actual grammar content
func (s *GrammarService) ResolveGrammar(grammarRef string) (string, error) {
	if grammarRef == "" {
		return "", nil
	}
	
	// If it contains grammar syntax, use directly
	if strings.Contains(grammarRef, "::=") {
		slog.Debug("Using inline grammar", "length", len(grammarRef))
		return grammarRef, nil
	}
	
	// Parse path reference (dir/name or just name)
	parts := strings.Split(grammarRef, "/")
	var dir, name string
	
	if len(parts) == 1 {
		dir = "default"
		name = parts[0]
	} else if len(parts) == 2 {
		dir = parts[0]
		name = parts[1]
	} else {
		return "", fmt.Errorf("invalid grammar reference format: %s", grammarRef)
	}
	
	grammar, err := s.repo.GetGrammar(dir, name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve grammar %s: %v", grammarRef, err)
	}
	
	slog.Info("Resolved grammar reference", "ref", grammarRef, "dir", dir, "name", name)
	return grammar.Grammar, nil
}

func (s *GrammarService) validateGrammarName(name string) error {
	if name == "" {
		return fmt.Errorf("grammar name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|") {
		return fmt.Errorf("grammar name contains invalid characters")
	}
	return nil
}

func (s *GrammarService) validateDirectoryName(dir string) error {
	if dir == "" {
		return nil // Empty is valid (will use default)
	}
	if strings.ContainsAny(dir, "\\:*?\"<>|") {
		return fmt.Errorf("directory name contains invalid characters")
	}
	if filepath.IsAbs(dir) {
		return fmt.Errorf("directory name cannot be absolute path")
	}
	return nil
}