package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Logger SQLiteロガー
type Logger struct {
	db        *sql.DB
	processID int
}

// NewLogger 新しいLoggerを作成
func NewLogger(dbPath string) (*Logger, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &Logger{
		db:        db,
		processID: os.Getpid(),
	}

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
			card_id TEXT,
			process_id INTEGER
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
			error_message TEXT,
			process_id INTEGER
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
	query := `INSERT INTO logs (level, message, reader_id, card_id, process_id) VALUES (?, ?, ?, ?, ?)`

	_, err := l.db.Exec(query, level, message, readerID, cardID, l.processID)
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
		(reader_id, card_id, card_type, atr, expiry_date, remain_count, felica_uid, status, error_message, process_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
		l.processID,
	)

	if err != nil {
		return fmt.Errorf("failed to insert read history: %w", err)
	}

	id, _ := result.LastInsertId()
	record.ID = id

	return nil
}

// LogEntry ログエントリ
type LogEntry struct {
	ID        int64
	Timestamp time.Time
	Level     string
	Message   string
	ReaderID  string
}

// GetLogs ログを取得（フィルタ付き）
func (l *Logger) GetLogs(readerID, level string, startTime, endTime int64, limit int32) ([]*LogEntry, int32, error) {
	// クエリ構築
	query := `SELECT id, timestamp, level, message, reader_id FROM logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM logs WHERE 1=1`
	args := []interface{}{}

	if readerID != "" {
		query += ` AND reader_id = ?`
		countQuery += ` AND reader_id = ?`
		args = append(args, readerID)
	}

	if level != "" {
		query += ` AND level = ?`
		countQuery += ` AND level = ?`
		args = append(args, level)
	}

	if startTime > 0 {
		query += ` AND strftime('%s', timestamp) >= ?`
		countQuery += ` AND strftime('%s', timestamp) >= ?`
		args = append(args, startTime)
	}

	if endTime > 0 {
		query += ` AND strftime('%s', timestamp) <= ?`
		countQuery += ` AND strftime('%s', timestamp) <= ?`
		args = append(args, endTime)
	}

	// 総件数を取得
	var totalCount int32
	err := l.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// ログを取得
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	} else {
		args = append(args, 100) // デフォルト100件
	}

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []*LogEntry

	for rows.Next() {
		entry := &LogEntry{}
		var timestamp string
		var readerID sql.NullString

		if err := rows.Scan(&entry.ID, &timestamp, &entry.Level, &entry.Message, &readerID); err != nil {
			return nil, 0, fmt.Errorf("failed to scan row: %w", err)
		}

		entry.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)
		if readerID.Valid {
			entry.ReaderID = readerID.String
		}

		logs = append(logs, entry)
	}

	return logs, totalCount, nil
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

// GetReadHistory 読み取り履歴を取得（フィルタ付き）
func (l *Logger) GetReadHistory(readerID, status string, startTime, endTime int64, limit int32) ([]*ReadHistoryRecord, int32, error) {
	// クエリ構築
	query := `SELECT id, timestamp, reader_id, card_id, card_type, atr,
		expiry_date, remain_count, felica_uid, status, error_message
		FROM read_history WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM read_history WHERE 1=1`
	args := []interface{}{}

	if readerID != "" {
		query += ` AND reader_id = ?`
		countQuery += ` AND reader_id = ?`
		args = append(args, readerID)
	}

	if status != "" {
		query += ` AND status = ?`
		countQuery += ` AND status = ?`
		args = append(args, status)
	}

	if startTime > 0 {
		query += ` AND strftime('%s', timestamp) >= ?`
		countQuery += ` AND strftime('%s', timestamp) >= ?`
		args = append(args, startTime)
	}

	if endTime > 0 {
		query += ` AND strftime('%s', timestamp) <= ?`
		countQuery += ` AND strftime('%s', timestamp) <= ?`
		args = append(args, endTime)
	}

	// 総件数を取得
	var totalCount int32
	err := l.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count read history: %w", err)
	}

	// 履歴を取得
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	} else {
		args = append(args, 100) // デフォルト100件
	}

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query read history: %w", err)
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
			return nil, 0, fmt.Errorf("failed to scan row: %w", err)
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

	return records, totalCount, nil
}
