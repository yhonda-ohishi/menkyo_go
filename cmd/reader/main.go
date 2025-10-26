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
	"sync"
	"syscall"
	"time"
	"unsafe"

	"menkyo_go/internal/config"
	"menkyo_go/internal/database"
	"menkyo_go/internal/nfc"
	"menkyo_go/internal/woffcl"
	"menkyo_go/internal/woffsv"
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
	log.Printf("Database path: %s", dbFullPath)

	// データベース初期化
	logger, err := database.NewLogger(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer logger.Close()

	logger.LogMessage("INFO", "License reader started")

	// woff-cl/woff-svクライアント初期化（スレッドセーフ）
	var woffSvClient *woffsv.AuthClient
	var woffSvMutex sync.RWMutex
	var heartbeatTicker *time.Ticker

	// woff-svクライアントを設定する関数（DNSエラー時はリトライ）
	setWoffSvClient := func(backendURL string) error {
		maxRetries := 5
		retryDelay := 3 * time.Second

		for attempt := 1; attempt <= maxRetries; attempt++ {
			newClient, err := woffsv.NewAuthClient(backendURL, cfg.WoffClSecret)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Heartbeatで接続確認
			hb, err := newClient.Heartbeat()
			if err != nil {
				newClient.Close()

				// DNSエラーの場合はリトライ
				errMsg := err.Error()
				isDNSError := strings.Contains(errMsg, "no such host") ||
							 strings.Contains(errMsg, "lookup")

				if isDNSError && attempt < maxRetries {
					log.Printf("DNS resolution failed (attempt %d/%d), retrying in %v...", attempt, maxRetries, retryDelay)
					logger.LogMessage("WARNING", fmt.Sprintf("DNS resolution failed (attempt %d/%d)", attempt, maxRetries))
					time.Sleep(retryDelay)
					continue
				}

				return fmt.Errorf("heartbeat failed: %w", err)
			}

			log.Printf("Connected to woff-sv: %s (status: %s, version: %s)", backendURL, hb.Status, hb.Version)
			logger.LogMessage("INFO", fmt.Sprintf("woff-sv connected: url=%s, status=%s, version=%s", backendURL, hb.Status, hb.Version))

			// 古いクライアントを閉じて新しいクライアントに置き換え
			woffSvMutex.Lock()
			if woffSvClient != nil {
				woffSvClient.Close()
			}
			woffSvClient = newClient
			woffSvMutex.Unlock()

			return nil
		}

		return fmt.Errorf("failed to connect after %d attempts", maxRetries)
	}

	// woff-svクライアントを取得する関数（スレッドセーフ）
	getWoffSvClient := func() *woffsv.AuthClient {
		woffSvMutex.RLock()
		defer woffSvMutex.RUnlock()
		return woffSvClient
	}

	if cfg.WoffClEndpoint != "" && cfg.WoffClSecret != "" {
		log.Printf("Fetching backend URL from woff-cl: %s", cfg.WoffClEndpoint)
		woffClClient := woffcl.NewClient(cfg.WoffClEndpoint, cfg.WoffClSecret)
		backendURL, err := woffClClient.GetBackendURL()

		if err != nil {
			log.Printf("Warning: Failed to get backend URL from woff-cl: %v", err)
			logger.LogMessage("WARNING", fmt.Sprintf("Failed to get backend URL: %v", err))

			// woff-clにアクセスできない場合、localhostにフォールバック（オフライン対応）
			log.Printf("Falling back to localhost for offline mode")
			logger.LogMessage("INFO", "Falling back to localhost for offline mode")
			backendURL = "http://localhost:50051"

			if err := setWoffSvClient(backendURL); err != nil {
				log.Printf("Warning: Failed to connect to localhost: %v", err)
				logger.LogMessage("WARNING", fmt.Sprintf("Failed to connect to localhost: %v", err))
			} else {
				log.Printf("Connected to localhost (offline mode)")
				logger.LogMessage("INFO", "Connected to localhost (offline mode)")
			}
		} else {
			log.Printf("Backend URL: %s", backendURL)
			logger.LogMessage("INFO", fmt.Sprintf("Backend URL from woff-cl: %s", backendURL))

			// 初回接続
			if err := setWoffSvClient(backendURL); err != nil {
				log.Printf("Warning: Failed to connect to woff-sv: %v", err)
				logger.LogMessage("WARNING", fmt.Sprintf("Failed to connect to woff-sv: %v", err))
			}
		}

		// WebSocketでバックエンドURL変更を常に監視
		// （Heartbeat失敗時の再接続に必要）
		watcher := woffcl.NewBackendWatcher(
			cfg.WoffClEndpoint,
			cfg.WoffClSecret,
			func(newURL string) bool {
				log.Printf("Backend URL received: %s", newURL)
				logger.LogMessage("INFO", fmt.Sprintf("Backend URL received: %s", newURL))

				// 新しいURLでクライアントを再作成（Heartbeatで接続確認）
				if err := setWoffSvClient(newURL); err != nil {
					log.Printf("Failed to connect to backend: %v", err)
					logger.LogMessage("WARNING", fmt.Sprintf("Failed to connect to backend: %v", err))
					return false // 接続失敗、WebSocketを継続して待機
				}

				log.Printf("Successfully connected to backend: %s", newURL)
				logger.LogMessage("INFO", fmt.Sprintf("Successfully connected to backend: %s", newURL))
				return true // 接続成功
			},
			func(msg string) {
				log.Printf("[WebSocket] %s", msg)
				logger.LogMessage("DEBUG", msg)
			},
		)
		watcher.Start()
		defer watcher.Stop()

		// 定期的にHeartbeatを送信（30秒間隔）
		heartbeatTicker = time.NewTicker(30 * time.Second)
		defer heartbeatTicker.Stop()

		go func() {
			for range heartbeatTicker.C {
				client := getWoffSvClient()
				if client == nil {
					continue
				}

				hb, err := client.Heartbeat()
				if err != nil {
					log.Printf("Heartbeat failed: %v", err)
					logger.LogMessage("WARNING", fmt.Sprintf("Heartbeat failed: %v", err))
				} else {
					log.Printf("Heartbeat OK (status: %s, timestamp: %s)", hb.Status, hb.Timestamp)
					logger.LogMessage("DEBUG", fmt.Sprintf("Heartbeat OK: status=%s", hb.Status))
				}
			}
		}()
	} else {
		log.Println("woff-cl endpoint or secret not configured, skipping woff-sv integration")
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
			// エラーをログに記録
			logger.LogMessage("ERROR", err.Error())

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

		// woff-svにTimeCardを送信（スレッドセーフ）
		client := getWoffSvClient()
		if client != nil {
			driverID := int32(0) // 暫定的に0を使用
			state := "in" // 出勤
			machineIP := *readerID // Reader IDを使用

			timeCard, err := client.CreateTimeCard(driverID, data.CardID, state, machineIP)
			if err != nil {
				log.Printf("Failed to send time card to woff-sv: %v", err)
				logger.LogMessage("ERROR", fmt.Sprintf("Failed to send time card to woff-sv: %v", err))
			} else {
				log.Printf("Time card sent to woff-sv successfully: id=%d, state=%s", timeCard.Id, timeCard.State)
				logger.LogMessage("INFO", fmt.Sprintf("Time card sent to woff-sv: id=%d, card_id=%s, state=%s",
					timeCard.Id, data.CardID, state))
			}
		}
	})

	if err != nil {
		log.Fatalf("Monitor error: %v", err)
	}
}
