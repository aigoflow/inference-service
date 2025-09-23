package repository

import (
	"context"
	"time"

	"github.com/aigoflow/inference-service/internal/models"
	"github.com/aigoflow/inference-service/internal/store"
)

// SQLiteRepository implements Repository interface using SQLite
type SQLiteRepository struct {
	db             *store.DB
	grammarRepo    GrammarRepositoryInterface
	requestRepo    RequestRepositoryInterface
	eventRepo      EventRepositoryInterface
}

func NewSQLiteRepository(db *store.DB, grammarRoot string) Repository {
	grammarRepo := NewGrammarRepository(grammarRoot)
	requestRepo := &SQLiteRequestRepository{db: db}
	eventRepo := &SQLiteEventRepository{db: db}
	
	return &SQLiteRepository{
		db:          db,
		grammarRepo: grammarRepo,
		requestRepo: requestRepo,
		eventRepo:   eventRepo,
	}
}

func (r *SQLiteRepository) Grammar() GrammarRepositoryInterface {
	return r.grammarRepo
}

func (r *SQLiteRepository) Request() RequestRepositoryInterface {
	return r.requestRepo
}

func (r *SQLiteRepository) Event() EventRepositoryInterface {
	return r.eventRepo
}

// SQLiteRequestRepository handles request logging
type SQLiteRequestRepository struct {
	db *store.DB
}

func (r *SQLiteRequestRepository) LogRequest(ctx context.Context, req *models.RequestLog) error {
	r.db.Req(
		req.Timestamp,
		req.TraceID,
		req.ReqID,
		req.WorkerID,
		req.Source,
		req.ReplyTo,
		req.RawInput,
		req.FormattedInput,
		req.ResponseText,
		req.ParamsJSON,
		req.GrammarUsed,
		req.TokensIn,
		req.TokensOut,
		time.Duration(req.DurationMs)*time.Millisecond,
		req.Status,
		req.Error,
	)
	return nil
}

func (r *SQLiteRequestRepository) GetRequestLogs(ctx context.Context, limit int) ([]*models.RequestLog, error) {
	rows, err := r.db.Query(`SELECT ts,trace_id,req_id,source,reply_to,raw_input,formatted_input,response_text,input_len,params_json,grammar_used,tokens_in,tokens_out,dur_ms,status,error FROM requests ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []*models.RequestLog
	for rows.Next() {
		var log models.RequestLog
		var tsFloat float64
		
		if err := rows.Scan(
			&tsFloat, &log.TraceID, &log.ReqID, &log.Source, &log.ReplyTo,
			&log.RawInput, &log.FormattedInput, &log.ResponseText, &log.InputLen,
			&log.ParamsJSON, &log.GrammarUsed, &log.TokensIn, &log.TokensOut,
			&log.DurationMs, &log.Status, &log.Error,
		); err == nil {
			log.Timestamp = time.Unix(0, int64(tsFloat*1e9))
			logs = append(logs, &log)
		}
	}
	
	return logs, nil
}

// SQLiteEventRepository handles event logging
type SQLiteEventRepository struct {
	db *store.DB
}

func (r *SQLiteEventRepository) LogEvent(ctx context.Context, level, code, msg string, meta map[string]interface{}) error {
	r.db.Event(level, code, msg, meta)
	return nil
}