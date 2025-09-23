package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aigoflow/inference-service/internal/models"
)

type GrammarRepository struct {
	grammarRoot string
}

func NewGrammarRepository(grammarRoot string) *GrammarRepository {
	return &GrammarRepository{
		grammarRoot: grammarRoot,
	}
}

func (r *GrammarRepository) ensureDir(dir string) error {
	fullPath := filepath.Join(r.grammarRoot, dir)
	return os.MkdirAll(fullPath, 0755)
}

func (r *GrammarRepository) getGrammarPath(dir, name string) string {
	return filepath.Join(r.grammarRoot, dir, name+".gbnf")
}

// CreateGrammar creates a new grammar file
func (r *GrammarRepository) CreateGrammar(dir, name string, content, description string) (*models.Grammar, error) {
	if err := r.ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}
	
	grammarPath := r.getGrammarPath(dir, name)
	
	// Check if file already exists
	if _, err := os.Stat(grammarPath); err == nil {
		return nil, fmt.Errorf("grammar %s/%s already exists", dir, name)
	}
	
	// Write grammar file
	if err := os.WriteFile(grammarPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write grammar file: %v", err)
	}
	
	// Get file info for metadata
	info, err := os.Stat(grammarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	
	return &models.Grammar{
		Name:        name,
		Directory:   dir,
		Description: description,
		Grammar:     content,
		Size:        info.Size(),
		Created:     info.ModTime(),
		Modified:    info.ModTime(),
	}, nil
}

// GetGrammar retrieves a specific grammar
func (r *GrammarRepository) GetGrammar(dir, name string) (*models.Grammar, error) {
	grammarPath := r.getGrammarPath(dir, name)
	
	// Check if file exists
	info, err := os.Stat(grammarPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("grammar %s/%s not found", dir, name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to access grammar file: %v", err)
	}
	
	// Read grammar content
	content, err := os.ReadFile(grammarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read grammar file: %v", err)
	}
	
	return &models.Grammar{
		Name:        name,
		Directory:   dir,
		Description: "", // TODO: Could store description in metadata
		Grammar:     string(content),
		Size:        info.Size(),
		Created:     info.ModTime(), // Approximation
		Modified:    info.ModTime(),
	}, nil
}

// UpdateGrammar updates an existing grammar
func (r *GrammarRepository) UpdateGrammar(dir, name, content, description string) (*models.Grammar, error) {
	grammarPath := r.getGrammarPath(dir, name)
	
	// Check if file exists
	if _, err := os.Stat(grammarPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("grammar %s/%s not found", dir, name)
	}
	
	// Update grammar file
	if err := os.WriteFile(grammarPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to update grammar file: %v", err)
	}
	
	// Get updated file info
	info, err := os.Stat(grammarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	
	return &models.Grammar{
		Name:        name,
		Directory:   dir,
		Description: description,
		Grammar:     content,
		Size:        info.Size(),
		Modified:    info.ModTime(),
	}, nil
}

// DeleteGrammar removes a grammar file
func (r *GrammarRepository) DeleteGrammar(dir, name string) error {
	grammarPath := r.getGrammarPath(dir, name)
	
	if _, err := os.Stat(grammarPath); os.IsNotExist(err) {
		return fmt.Errorf("grammar %s/%s not found", dir, name)
	}
	
	return os.Remove(grammarPath)
}

// ListGrammars lists all grammars in a directory
func (r *GrammarRepository) ListGrammars(dir string) ([]*models.Grammar, error) {
	dirPath := filepath.Join(r.grammarRoot, dir)
	
	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return []*models.Grammar{}, nil // Empty list for non-existent directory
	}
	
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}
	
	var grammars []*models.Grammar
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gbnf") {
			continue
		}
		
		name := strings.TrimSuffix(entry.Name(), ".gbnf")
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// Read content for full grammar object
		content, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			continue
		}
		
		grammars = append(grammars, &models.Grammar{
			Name:      name,
			Directory: dir,
			Grammar:   string(content),
			Size:      info.Size(),
			Created:   info.ModTime(),
			Modified:  info.ModTime(),
		})
	}
	
	return grammars, nil
}

// ListDirectories lists all available grammar directories
func (r *GrammarRepository) ListDirectories() ([]string, error) {
	entries, err := os.ReadDir(r.grammarRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read grammar root: %v", err)
	}
	
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	
	return dirs, nil
}

// CreateDirectory creates a new grammar directory
func (r *GrammarRepository) CreateDirectory(name string) error {
	dirPath := filepath.Join(r.grammarRoot, name)
	
	// Check if directory already exists
	if _, err := os.Stat(dirPath); err == nil {
		return fmt.Errorf("directory %s already exists", name)
	}
	
	return os.MkdirAll(dirPath, 0755)
}

// DeleteDirectory removes a grammar directory and all its contents
func (r *GrammarRepository) DeleteDirectory(name string) error {
	dirPath := filepath.Join(r.grammarRoot, name)
	
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("directory %s not found", name)
	}
	
	return os.RemoveAll(dirPath)
}