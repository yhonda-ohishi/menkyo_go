package license

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "menkyo_go/proto/license"
	"menkyo_go/internal/database"

	"github.com/google/uuid"
)

// Server gRPCサーバー
type Server struct {
	pb.UnimplementedLicenseReaderServer
	logger   *database.Logger
	callback func(*pb.LicenseData)
}

// NewServer 新しいServerを作成
func NewServer(logger *database.Logger, callback func(*pb.LicenseData)) *Server {
	return &Server{
		logger:   logger,
		callback: callback,
	}
}

// PushLicenseData 免許証データを受信
func (s *Server) PushLicenseData(ctx context.Context, data *pb.LicenseData) (*pb.PushResponse, error) {
	requestID := uuid.New().String()

	log.Printf("[%s] Received license data: CardID=%s, Type=%s",
		requestID, data.CardId, data.LicenseType)

	// データベースに記録
	if s.logger != nil {
		record := &database.ReadHistoryRecord{
			ReaderID:    data.ReaderId,
			CardID:      data.CardId,
			CardType:    "driver_license",
			ATR:         "", // ATRは含まれていない
			ExpiryDate:  data.ExpiryDate,
			RemainCount: "",
			FeliCaUID:   "",
			Status:      "success",
			Timestamp:   time.Unix(data.ReadTimestamp, 0),
		}

		if err := s.logger.LogReadHistory(record); err != nil {
			log.Printf("[%s] Failed to log read history: %v", requestID, err)
		}

		s.logger.LogMessage("INFO", fmt.Sprintf("Received license data: %s", data.CardId))
	}

	// コールバックを実行
	if s.callback != nil {
		s.callback(data)
	}

	return &pb.PushResponse{
		Success:   true,
		Message:   "License data received successfully",
		RequestId: requestID,
	}, nil
}

// PushReadLog 読み取りログを受信
func (s *Server) PushReadLog(ctx context.Context, logData *pb.ReadLog) (*pb.PushResponse, error) {
	requestID := uuid.New().String()

	log.Printf("[%s] Received read log: ReaderID=%s, Status=%s",
		requestID, logData.ReaderId, logData.Status)

	// データベースに記録
	if s.logger != nil {
		level := "INFO"
		if logData.Status == "error" {
			level = "ERROR"
		}

		message := fmt.Sprintf("Reader %s: %s", logData.ReaderId, logData.Status)
		if logData.ErrorMessage != "" {
			message += fmt.Sprintf(" - %s", logData.ErrorMessage)
		}

		if err := s.logger.LogMessageWithContext(level, message, logData.ReaderId, logData.CardId); err != nil {
			log.Printf("[%s] Failed to log message: %v", requestID, err)
		}
	}

	return &pb.PushResponse{
		Success:   true,
		Message:   "Read log received successfully",
		RequestId: requestID,
	}, nil
}

// TODO: GetLogs/GetReadHistory - protoファイルを更新して再生成後に有効化
/*
// GetLogs ログを取得
func (s *Server) GetLogs(ctx context.Context, req *pb.GetLogsRequest) (*pb.GetLogsResponse, error) {
	if s.logger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}

	// データベースからログを取得
	logs, totalCount, err := s.logger.GetLogs(
		req.ReaderId,
		req.Level,
		req.StartTime,
		req.EndTime,
		req.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// proto形式に変換
	pbLogs := make([]*pb.LogEntry, len(logs))
	for i, logEntry := range logs {
		pbLogs[i] = &pb.LogEntry{
			Timestamp: logEntry.Timestamp.Unix(),
			Level:     logEntry.Level,
			Message:   logEntry.Message,
			ReaderId:  logEntry.ReaderID,
		}
	}

	return &pb.GetLogsResponse{
		Logs:       pbLogs,
		TotalCount: totalCount,
	}, nil
}

// GetReadHistory 読み取り履歴を取得
func (s *Server) GetReadHistory(ctx context.Context, req *pb.GetReadHistoryRequest) (*pb.GetReadHistoryResponse, error) {
	if s.logger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}

	// データベースから履歴を取得
	records, totalCount, err := s.logger.GetReadHistory(
		req.ReaderId,
		req.Status,
		req.StartTime,
		req.EndTime,
		req.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get read history: %w", err)
	}

	// proto形式に変換
	pbEntries := make([]*pb.ReadHistoryEntry, len(records))
	for i, record := range records {
		pbEntries[i] = &pb.ReadHistoryEntry{
			Timestamp:    record.Timestamp.Unix(),
			ReaderId:     record.ReaderID,
			CardId:       record.CardID,
			CardType:     record.CardType,
			Atr:          record.ATR,
			ExpiryDate:   record.ExpiryDate,
			RemainCount:  record.RemainCount,
			FelicaUid:    record.FeliCaUID,
			Status:       record.Status,
			ErrorMessage: record.ErrorMessage,
		}
	}

	return &pb.GetReadHistoryResponse{
		Entries:    pbEntries,
		TotalCount: totalCount,
	}, nil
}
*/
