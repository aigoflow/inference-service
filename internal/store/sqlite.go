package store

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create events table
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS events(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts REAL,
		level TEXT,
		code TEXT,
		msg TEXT,
		meta TEXT
	)`); err != nil {
		return nil, err
	}

	// Create requests table with full request/response content
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS requests(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts REAL,
		trace_id TEXT,
		req_id TEXT,
		source TEXT,
		reply_to TEXT,
		raw_input TEXT,
		formatted_input TEXT,
		input_len INTEGER,
		params_json TEXT,
		grammar_used TEXT,
		response_text TEXT,
		tokens_in INTEGER,
		tokens_out INTEGER,
		dur_ms REAL,
		status TEXT,
		error TEXT
	)`); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) Event(level, code, msg string, meta map[string]interface{}) {
	m := ""
	if meta != nil {
		b, _ := json.Marshal(meta)
		m = string(b)
	}
	_, _ = db.Exec(`INSERT INTO events(ts,level,code,msg,meta) VALUES(?,?,?,?,?)`,
		float64(time.Now().UnixNano())/1e9, level, code, msg, m)
}

func (db *DB) Req(start time.Time, traceID, reqID, source, replyTo, rawInput, formattedInput, responseText, params, grammarUsed string,
	tokIn, tokOut int, dur time.Duration, status, errStr string) {
	_, _ = db.Exec(`INSERT INTO requests(
		ts, trace_id, req_id, source, reply_to, raw_input, formatted_input, input_len, params_json, grammar_used, response_text, tokens_in, tokens_out, dur_ms, status, error)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		float64(start.UnixNano())/1e9, traceID, reqID, source, replyTo, rawInput, formattedInput, len(rawInput), params, grammarUsed, responseText, tokIn, tokOut, float64(dur.Milliseconds()), status, errStr)
}