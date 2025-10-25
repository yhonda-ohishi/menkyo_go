package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// ServerConfig サーバー設定
type ServerConfig struct {
	Port   int
	DBPath string
}

// ReaderConfig リーダー設定
type ReaderConfig struct {
	ServerAddr string
	DBPath     string
	ReaderID   string
}

// LoadEnv 環境変数を読み込む
func LoadEnv(envFile string) error {
	// .envファイルが存在する場合のみ読み込む
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}
	return nil
}

// GetServerConfig サーバー設定を取得
func GetServerConfig() *ServerConfig {
	config := &ServerConfig{
		Port:   50051,
		DBPath: "license_server.db",
	}

	// 環境変数から取得
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	if dbPath := os.Getenv("SERVER_DB_PATH"); dbPath != "" {
		config.DBPath = dbPath
	}

	return config
}

// GetReaderConfig リーダー設定を取得
func GetReaderConfig() *ReaderConfig {
	config := &ReaderConfig{
		ServerAddr: "localhost:50051",
		DBPath:     "license_reader.db",
		ReaderID:   "default",
	}

	// 環境変数から取得
	if serverAddr := os.Getenv("GRPC_SERVER_ADDR"); serverAddr != "" {
		config.ServerAddr = serverAddr
	}

	if dbPath := os.Getenv("READER_DB_PATH"); dbPath != "" {
		config.DBPath = dbPath
	}

	if readerID := os.Getenv("READER_ID"); readerID != "" {
		config.ReaderID = readerID
	}

	return config
}
