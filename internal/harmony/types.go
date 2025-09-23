package harmony

import (
	"time"
)

// Role represents the different message roles in Harmony format
type Role string

const (
	RoleSystem     Role = "system"
	RoleDeveloper  Role = "developer"
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleTool       Role = "tool"
)

// ReasoningLevel controls the reasoning effort
type ReasoningLevel string

const (
	ReasoningLow    ReasoningLevel = "low"
	ReasoningMedium ReasoningLevel = "medium"
	ReasoningHigh   ReasoningLevel = "high"
)

// Channel represents output channels in Harmony format
type Channel string

const (
	ChannelAnalysis    Channel = "analysis"
	ChannelCommentary  Channel = "commentary"
	ChannelFinal       Channel = "final"
)

// Message represents a single message in a conversation
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	Channel Channel `json:"channel,omitempty"`
}

// Tool represents a function/tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// SystemConfig holds system-level configuration
type SystemConfig struct {
	ModelIdentity    string         `json:"model_identity"`
	KnowledgeCutoff  string         `json:"knowledge_cutoff"`
	CurrentDate      string         `json:"current_date"`
	ReasoningLevel   ReasoningLevel `json:"reasoning_level"`
	ValidChannels    []Channel      `json:"valid_channels"`
}

// Conversation represents a complete Harmony conversation
type Conversation struct {
	SystemConfig SystemConfig `json:"system_config"`
	Messages     []Message    `json:"messages"`
	Tools        []Tool       `json:"tools,omitempty"`
}

// ConversationBuilder helps build Harmony conversations
type ConversationBuilder struct {
	conversation *Conversation
}

// NewConversationBuilder creates a new conversation builder
func NewConversationBuilder() *ConversationBuilder {
	return &ConversationBuilder{
		conversation: &Conversation{
			SystemConfig: SystemConfig{
				ModelIdentity:   "ChatGPT, a large language model trained by OpenAI",
				KnowledgeCutoff: "2024-06",
				CurrentDate:     time.Now().Format("2006-01-02"),
				ReasoningLevel:  ReasoningMedium,
				ValidChannels:   []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
			},
			Messages: []Message{},
			Tools:    []Tool{},
		},
	}
}

// WithReasoningLevel sets the reasoning level
func (cb *ConversationBuilder) WithReasoningLevel(level ReasoningLevel) *ConversationBuilder {
	cb.conversation.SystemConfig.ReasoningLevel = level
	return cb
}

// WithModelIdentity sets the model identity
func (cb *ConversationBuilder) WithModelIdentity(identity string) *ConversationBuilder {
	cb.conversation.SystemConfig.ModelIdentity = identity
	return cb
}

// WithTools adds tools to the conversation
func (cb *ConversationBuilder) WithTools(tools []Tool) *ConversationBuilder {
	cb.conversation.Tools = tools
	return cb
}

// AddMessage adds a message to the conversation
func (cb *ConversationBuilder) AddMessage(role Role, content string) *ConversationBuilder {
	cb.conversation.Messages = append(cb.conversation.Messages, Message{
		Role:    role,
		Content: content,
	})
	return cb
}

// AddSystemMessage adds a system message
func (cb *ConversationBuilder) AddSystemMessage(content string) *ConversationBuilder {
	return cb.AddMessage(RoleSystem, content)
}

// AddDeveloperMessage adds a developer message
func (cb *ConversationBuilder) AddDeveloperMessage(content string) *ConversationBuilder {
	return cb.AddMessage(RoleDeveloper, content)
}

// AddUserMessage adds a user message
func (cb *ConversationBuilder) AddUserMessage(content string) *ConversationBuilder {
	return cb.AddMessage(RoleUser, content)
}

// Build returns the constructed conversation
func (cb *ConversationBuilder) Build() *Conversation {
	return cb.conversation
}