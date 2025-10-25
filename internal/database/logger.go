package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Logger SQLiteロガー
type Logger struct {
	db *sql.DB
}

// NewLogger 新しいLoggerを作成
func NewLogger(dbPath string) (*Logger, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &Logger{db: db}

	// テーブルを初期化
	if err := logger.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return logger, nil
}

// Close データベースを閉じる
func (l *Logger) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// initTables テーブルを初期化
func (l *Logger) initTables() error {
	queries := []string{
		// ログテーブル
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			reader_id TEXT,
			card_id TEXT
		)`,
		// 読み取り履歴テーブル
		`CREATE TABLE IF NOT EXISTS read_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			reader_id TEXT NOT NULL,
			card_id TEXT NOT NULL,
			card_type TEXT NOT NULL,
			atr TEXT,
			expiry_date TEXT,
			remain_count TEXT,
			felica_uid TEXT,
			status TEXT NOT NULL,
			error_message TEXT
		)`,
		// インデックス
		`CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_card_id ON logs(card_id)`,
		`CREATE INDEX IF NOT EXISTS idx_read_history_timestamp ON read_history(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_read_history_card_id ON read_history(card_id)`,
	}

	for _, query := range queries {
		if _, err := l.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// LogMessage メッセージをログに記録
func (l *Logger) LogMessage(level, message string) error {
	return l.LogMessageWithContext(level, message, "", "")
}

// LogMessageWithContext コンテキスト付きでメッセージをログに記録
func (l *Logger) LogMessageWithContext(level, message, readerID, cardID string) error {
	query := `INSERT INTO logs (level, message, reader_id, card_id) VALUES (?, ?, ?, ?)`

	_, err := l.db.Exec(query, level, message, readerID, cardID)
	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}

// ReadHistoryRecord 読み取り履歴レコード
type ReadHistoryRecord struct {
	ID           int64
	Timestamp    time.Time
	ReaderID     string
	CardID       string
	CardType     string
	ATR          string
	ExpiryDate   string
	RemainCount  string
	FeliCaUID    string
	Status       string
	ErrorMessage string
}

// LogReadHistory 読み取り履歴を記録
func (l *Logger) LogReadHistory(record *ReadHistoryRecord) error {
	query := `INSERT INTO read_history
		(reader_id, card_id, card_type, atr, expiry_date, remain_count, felica_uid, status, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := l.db.Exec(query,
		record.ReaderID,
		record.CardID,
		record.CardType,
		record.ATR,
		record.ExpiryDate,
		record.RemainCount,
		record.FeliCaUID,
		record.Status,
		record.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to insert read history: %w", err)
	}

	id, _ := result.LastInsertId()
	record.ID = id

	return nil
}

// GetRecentLogs 最近のログを取得
func (l *Logger) GetRecentLogs(limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, timestamp, level, message, reader_id, card_id
		FROM logs
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := l.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []map[string]interface{}

	for rows.Next() {
		var id int64
		var timestamp string
		var level, message string
		var readerID, cardID sql.NullString

		if err := rows.Scan(&id, &timestamp, &level, &message, &readerID, &cardID); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		log := map[string]interface{}{
			"id":        id,
			"timestamp": timestamp,
			"level":     level,
			"message":   message,
		}

		if readerID.Valid {
			log["reader_id"] = readerID.String
		}
		if cardID.Valid {
			log["card_id"] = cardID.String
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetReadHistory 読み取り履歴を取得
func (l *Logger) GetReadHistory(limit int) ([]*ReadHistoryRecord, error) {
	query := `SELECT id, timestamp, reader_id, card_id, card_type, atr,
		expiry_date, remain_count, felica_uid, status, error_message
		FROM read_history
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := l.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query read history: %w", err)
	}
	defer rows.Close()

	var records []*ReadHistoryRecord

	for rows.Next() {
		record := &ReadHistoryRecord{}
		var timestamp string
		var expiryDate, remainCount, felicaUID, errorMessage sql.NullString

		if err := rows.Scan(
			&record.ID,
			&timestamp,
			&record.ReaderID,
			&record.CardID,
			&record.CardType,
			&record.ATR,
			&expiryDate,
			&remainCount,
			&felicaUID,
			&record.Status,
			&errorMessage,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		record.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)

		if expiryDate.Valid {
			record.ExpiryDate = expiryDate.String
		}
		if remainCount.Valid {
			record.RemainCount = remainCount.String
		}
		if felicaUID.Valid {
			record.FeliCaUID = felicaUID.String
		}
		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}

		records = append(records, record)
	}

	return records, nil
}
