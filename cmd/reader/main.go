// +build windows

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"menkyo_go/internal/config"
	"menkyo_go/internal/database"
	"menkyo_go/internal/license"
	"menkyo_go/internal/nfc"
	pb "menkyo_go/proto"
)

var (
	// ビルド時に埋め込まれる変数
	Version   = "dev"
	BuildTime = "unknown"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex    = kernel32.NewProc("CreateMutexW")
	procReleaseMutex   = kernel32.NewProc("ReleaseMutex")
	procGetLastError   = kernel32.NewProc("GetLastError")
)

const (
	ERROR_ALREADY_EXISTS = 183
)

func killOldReaderProcesses() {
	log.Println("Killing all existing reader.exe processes...")
	cmd := exec.Command("taskkill", "/F", "/IM", "reader.exe")
	output, _ := cmd.CombinedOutput()
	log.Printf("taskkill output: %s", output)
	time.Sleep(500 * time.Millisecond) // プロセス終了を待つ
}

func main() {
	// 多重起動チェック（Windows Mutex）
	mutexName, _ := syscall.UTF16PtrFromString("Global\\MenkyoReaderMutex")
	r1, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if r1 == 0 {
		log.Fatalf("Failed to create mutex")
	}
	mutex := syscall.Handle(r1)
	defer syscall.CloseHandle(mutex)

	// errがERROR_ALREADY_EXISTSかチェック
	if err.(syscall.Errno) == ERROR_ALREADY_EXISTS {
		log.Println("Another instance detected. Killing old processes...")
		syscall.CloseHandle(mutex) // 一旦Mutexを閉じる

		killOldReaderProcesses()

		// Mutexが完全にリリースされるまで待機してリトライ
		for i := 0; i < 10; i++ {
			time.Sleep(200 * time.Millisecond)
			r1, _, err = procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
			if r1 == 0 {
				continue
			}
			mutex = syscall.Handle(r1)
			if err.(syscall.Errno) != ERROR_ALREADY_EXISTS {
				log.Println("Successfully acquired mutex after killing old processes")
				break
			}
			syscall.CloseHandle(mutex)
		}
		if err.(syscall.Errno) == ERROR_ALREADY_EXISTS {
			log.Fatalf("Failed to acquire mutex after killing old processes")
		}
	}

	defer procReleaseMutex.Call(uintptr(mutex))

	// ログファイル設定（logフォルダに日付ごと）
	logDir := "log"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: failed to create log directory: %v", err)
	}

	// 日付ごとのログファイル名（例: log/reader_20251026.log）
	logFileName := fmt.Sprintf("%s/reader_%s.log", logDir, time.Now().Format("20060102"))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		defer logFile.Close()
		// コンソールとファイルの両方に出力
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)
		log.Printf("Log file: %s", logFileName)
	}

	// .envファイルを読み込む
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: %v", err)
	}

	// 環境変数からデフォルト値を取得
	cfg := config.GetReaderConfig()

	// コマンドラインフラグ（環境変数より優先される）
	serverAddr := flag.String("server", cfg.ServerAddr, "gRPC server address")
	dbPath := flag.String("db", cfg.DBPath, "SQLite database path")
	readerID := flag.String("reader-id", cfg.ReaderID, "Reader ID")
	flag.Parse()

	// データベースのフルパスを取得
	dbFullPath := *dbPath
	if !strings.HasPrefix(*dbPath, "/") && !strings.Contains(*dbPath, ":\\") {
		if cwd, err := os.Getwd(); err == nil {
			dbFullPath = cwd + "\\" + *dbPath
		}
	}

	log.Printf("Starting license reader v%s (Built: %s)", Version, BuildTime)
	log.Printf("Reader ID: %s", *readerID)
	log.Printf("Server address: %s", *serverAddr)
	log.Printf("Database path: %s", dbFullPath)

	// データベース初期化
	logger, err := database.NewLogger(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer logger.Close()

	logger.LogMessage("INFO", "License reader started")

	// gRPCクライアント初期化
	grpcClient, err := license.NewClient(*serverAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to gRPC server: %v", err)
		logger.LogMessage("WARNING", "Failed to connect to gRPC server")
		// gRPCサーバーに接続できなくてもローカルログは動作させる
		grpcClient = nil
	} else {
		defer grpcClient.Close()
		log.Printf("Connected to gRPC server: %s", *serverAddr)
		logger.LogMessage("INFO", "Connected to gRPC server")
	}

	// NFC リーダー初期化（内部ログはログファイルとDBの両方に出力）
	licenseReader, err := nfc.NewLicenseReader(func(msg string) {
		// ログファイルとDBの両方に記録
		log.Printf("[NFC] %s", msg)
		logger.LogMessage("DEBUG", msg)
	})
	if err != nil {
		log.Fatalf("Failed to initialize license reader: %v", err)
	}
	defer licenseReader.Close()

	// リーダーをリスト
	readers, err := licenseReader.ListReaders()
	if err != nil {
		log.Fatalf("Failed to list readers: %v", err)
	}

	if len(readers) == 0 {
		log.Fatalf("No NFC readers found")
	}

	log.Printf("Found %d reader(s):", len(readers))
	for i, reader := range readers {
		log.Printf("  [%d] %s", i, reader)
	}

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down...")
		logger.LogMessage("INFO", "License reader stopped")
		os.Exit(0)
	}()

	// カード監視開始
	log.Println("Monitoring for cards... (Press Ctrl+C to exit)")
	logger.LogMessage("INFO", "Started monitoring for cards")

	err = licenseReader.MonitorCards(func(data *nfc.LicenseData, err error) {
		if err != nil {
			// エラーはDBのみに記録（コンソールには出さない）
			logger.LogMessage("ERROR", err.Error())

			// エラーログをgRPCサーバーに送信
			if grpcClient != nil {
				timestamp := int64(0)
				if data != nil {
					timestamp = data.ReadTimestamp.Unix()
				}
				logData := &pb.ReadLog{
					Timestamp:    timestamp,
					ReaderId:     *readerID,
					Status:       "error",
					ErrorMessage: err.Error(),
				}
				if data != nil {
					logData.CardId = data.CardID
				}
				grpcClient.PushReadLog(logData)
			}

			// データベースに記録
			record := &database.ReadHistoryRecord{
				ReaderID:     *readerID,
				CardID:       "",
				CardType:     "",
				Status:       "error",
				ErrorMessage: err.Error(),
			}
			logger.LogReadHistory(record)

			return
		}

		// Expiry DateとFeliCa UIDのみ表示
		if data.ExpiryDate != "" {
			log.Printf("Expiry Date: %s", data.ExpiryDate)
		}
		if data.FeliCaUID != "" {
			log.Printf("FeliCa UID: %s", data.FeliCaUID)
		}
		if data.ExpiryDate == "" && data.FeliCaUID == "" {
			log.Printf("Card Type: %s (No expiry date or FeliCa UID)", data.CardType)
		}

		// データベースに記録
		record := &database.ReadHistoryRecord{
			ReaderID:    *readerID,
			CardID:      data.CardID,
			CardType:    data.CardType,
			ATR:         data.ATR,
			ExpiryDate:  data.ExpiryDate,
			RemainCount: data.RemainCount,
			FeliCaUID:   data.FeliCaUID,
			Status:      "success",
			Timestamp:   data.ReadTimestamp,
		}
		if err := logger.LogReadHistory(record); err != nil {
			log.Printf("Failed to log read history: %v", err)
		}

		// gRPCサーバーにデータを送信
		if grpcClient != nil {
			licenseData := &pb.LicenseData{
				CardId:        data.CardID,
				LicenseType:   data.CardType,
				ExpiryDate:    data.ExpiryDate,
				ReadTimestamp: data.ReadTimestamp.Unix(),
				ReaderId:      *readerID,
			}

			resp, err := grpcClient.PushLicenseData(licenseData)
			if err != nil {
				log.Printf("Failed to push license data to server: %v", err)
				logger.LogMessage("ERROR", "Failed to push data to server")
			} else {
				log.Printf("Data pushed to server (Request ID: %s)", resp.RequestId)
			}

			// 成功ログを送信
			logData := &pb.ReadLog{
				Timestamp: data.ReadTimestamp.Unix(),
				ReaderId:  *readerID,
				Status:    "success",
				CardId:    data.CardID,
			}
			grpcClient.PushReadLog(logData)
		}
	})

	if err != nil {
		log.Fatalf("Monitor error: %v", err)
	}
}
