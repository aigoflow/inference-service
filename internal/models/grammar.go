package models

import (
	"time"
)

// Grammar represents a grammar file for constrained generation
type Grammar struct {
	Name        string    `json:"name"`
	Directory   string    `json:"directory"`
	Description string    `json:"description"`
	Grammar     string    `json:"grammar"`
	Size        int64     `json:"size"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
}

// CreateGrammarRequest represents a request to create a new grammar
type CreateGrammarRequest struct {
	Name        string `json:"name" validate:"required"`
	Directory   string `json:"directory"`
	Description string `json:"description"`
	Grammar     string `json:"grammar" validate:"required"`
}

// UpdateGrammarRequest represents a request to update an existing grammar
type UpdateGrammarRequest struct {
	Description string `json:"description"`
	Grammar     string `json:"grammar" validate:"required"`
}

// GrammarListResponse represents the response for listing grammars
type GrammarListResponse struct {
	Directory string     `json:"directory"`
	Grammars  []*Grammar `json:"grammars"`
}

// DirectoryListResponse represents the response for listing directories
type DirectoryListResponse struct {
	Directories []string `json:"directories"`
}

// CreateDirectoryRequest represents a request to create a directory
type CreateDirectoryRequest struct {
	Name string `json:"name" validate:"required"`
}

