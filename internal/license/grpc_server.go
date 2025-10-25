package license

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "menkyo_go/proto"
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
