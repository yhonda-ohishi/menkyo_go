// +build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"menkyo_go/internal/config"
	"menkyo_go/internal/database"
)

func main() {
	// .envファイルを読み込む
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: %v", err)
	}

	// 環境変数からデフォルト値を取得
	cfg := config.GetReaderConfig()

	// コマンドラインフラグ
	readerPath := flag.String("reader", "reader.exe", "Path to reader.exe")
	readerID := flag.String("reader-id", cfg.ReaderID, "Reader ID")
	dbPath := flag.String("db", "supervisor.db", "Supervisor database path")
	restartDelay := flag.Duration("restart-delay", 5*time.Second, "Delay before restarting reader")
	flag.Parse()

	log.Printf("Starting reader supervisor")
	log.Printf("Reader path: %s", *readerPath)
	log.Printf("Reader ID: %s", *readerID)
	log.Printf("Restart delay: %v", *restartDelay)

	// データベース初期化（ログ用）
	logger, err := database.NewLogger(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer logger.Close()

	logger.LogMessage("INFO", "Reader supervisor started")

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	stopChan := make(chan struct{})
	go func() {
		<-sigChan
		log.Println("\nShutting down supervisor...")
		logger.LogMessage("INFO", "Reader supervisor stopped")
		close(stopChan)
	}()

	// readerプロセスを監視・再起動（無制限）
	restartCount := 0
	for {
		select {
		case <-stopChan:
			log.Println("Supervisor stopped")
			return
		default:
			if restartCount > 0 {
				log.Printf("Restarting reader (attempt %d) in %v...", restartCount, *restartDelay)
				logger.LogMessage("WARNING", fmt.Sprintf("Restarting reader (attempt %d)", restartCount))
				time.Sleep(*restartDelay)
			}

			log.Printf("Starting reader process...")
			logger.LogMessage("INFO", "Starting reader process")

			// reader.exeを起動（supervisorと同じディレクトリを基準）
			execPath, err := os.Executable()
			if err != nil {
				log.Printf("Failed to get executable path: %v", err)
				logger.LogMessage("ERROR", fmt.Sprintf("Failed to get executable path: %v", err))
				restartCount++
				continue
			}
			execDir := filepath.Dir(execPath)
			readerExePath := filepath.Join(execDir, *readerPath)

			cmd := exec.Command(readerExePath, "--reader-id", *readerID)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Start(); err != nil {
				log.Printf("Failed to start reader: %v", err)
				logger.LogMessage("ERROR", fmt.Sprintf("Failed to start reader: %v", err))
				restartCount++
				continue
			}

			log.Printf("Reader process started (PID: %d)", cmd.Process.Pid)
			logger.LogMessage("INFO", fmt.Sprintf("Reader process started (PID: %d)", cmd.Process.Pid))

			// プロセス終了を待機
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-stopChan:
				// 停止シグナルを受信した場合、readerを終了
				log.Println("Terminating reader process...")
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("Failed to kill reader: %v", err)
				}
				return
			case err := <-done:
				if err != nil {
					log.Printf("Reader process exited with error: %v", err)
					logger.LogMessage("ERROR", fmt.Sprintf("Reader process exited with error: %v", err))
					restartCount++
				} else {
					log.Printf("Reader process exited normally")
					logger.LogMessage("INFO", "Reader process exited normally")
					// 正常終了の場合は再起動カウントをリセット
					restartCount = 0
				}
			}
		}
	}
}
