package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"menkyo_go/internal/config"
	"menkyo_go/internal/database"
	"menkyo_go/internal/license"
	pb "menkyo_go/proto"

	"google.golang.org/grpc"
)

func main() {
	// .envファイルを読み込む
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: %v", err)
	}

	// 環境変数からデフォルト値を取得
	cfg := config.GetServerConfig()

	// コマンドラインフラグ（環境変数より優先される）
	port := flag.Int("port", cfg.Port, "gRPC server port")
	dbPath := flag.String("db", cfg.DBPath, "SQLite database path")
	flag.Parse()

	log.Printf("Starting license server on port %d", *port)
	log.Printf("Database path: %s", *dbPath)

	// データベース初期化
	logger, err := database.NewLogger(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer logger.Close()

	logger.LogMessage("INFO", "License server started")

	// gRPCサーバー作成
	licenseServer := license.NewServer(logger, func(data *pb.LicenseData) {
		log.Printf("Callback: Received license data - CardID: %s, Type: %s",
			data.CardId, data.LicenseType)
		// ここで追加の処理を実行できます
		// 例: 別のシステムに通知、Webhookの送信など
	})

	grpcServer := grpc.NewServer()
	pb.RegisterLicenseReaderServer(grpcServer, licenseServer)

	// リスナーを作成
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down server...")
		logger.LogMessage("INFO", "License server stopped")
		grpcServer.GracefulStop()
		os.Exit(0)
	}()

	// サーバー起動
	log.Printf("Server listening on :%d", *port)
	logger.LogMessage("INFO", fmt.Sprintf("Server listening on port %d", *port))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
