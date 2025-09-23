package models

import "time"

// RequestLog represents a logged inference request
type RequestLog struct {
	Timestamp      time.Time `json:"ts"`
	TraceID        string    `json:"trace_id"`
	ReqID          string    `json:"req_id"`
	WorkerID       string    `json:"worker_id"`
	Source         string    `json:"source"`
	ReplyTo        string    `json:"reply_to"`
	RawInput       string    `json:"raw_input"`
	FormattedInput string    `json:"formatted_input"`
	ResponseText   string    `json:"response_text"`
	InputLen       int       `json:"input_len"`
	ParamsJSON     string    `json:"params_json"`
	GrammarUsed    string    `json:"grammar_used"`
	TokensIn       int       `json:"tokens_in"`
	TokensOut      int       `json:"tokens_out"`
	DurationMs     float64   `json:"dur_ms"`
	Status         string    `json:"status"`
	Error          string    `json:"error"`
}