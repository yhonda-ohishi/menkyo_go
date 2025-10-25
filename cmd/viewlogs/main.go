package main

import (
	"flag"
	"fmt"
	"log"

	"menkyo_go/internal/database"
)

func main() {
	dbPath := flag.String("db", "license_server.db", "Database file path")
	limit := flag.Int("limit", 30, "Number of logs to display")
	flag.Parse()

	logger, err := database.NewLogger(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer logger.Close()

	fmt.Printf("=== Recent Logs from %s ===\n\n", *dbPath)

	logs, err := logger.GetRecentLogs(*limit)
	if err != nil {
		log.Fatalf("Failed to get logs: %v", err)
	}

	for _, logEntry := range logs {
		fmt.Printf("[%s] %s: %s\n",
			logEntry["timestamp"],
			logEntry["level"],
			logEntry["message"])
		if readerID, ok := logEntry["reader_id"].(string); ok && readerID != "" {
			fmt.Printf("  Reader: %s\n", readerID)
		}
		if cardID, ok := logEntry["card_id"].(string); ok && cardID != "" {
			fmt.Printf("  Card: %s\n", cardID)
		}
		fmt.Println()
	}

	fmt.Println("=== Read History ===\n")

	history, err := logger.GetReadHistory(*limit)
	if err != nil {
		log.Fatalf("Failed to get read history: %v", err)
	}

	for _, record := range history {
		fmt.Printf("[%s] %s - %s\n",
			record.Timestamp.Format("2006-01-02 15:04:05"),
			record.ReaderID,
			record.Status)
		fmt.Printf("  Card ID: %s\n", record.CardID)
		fmt.Printf("  Card Type: %s\n", record.CardType)
		if record.ExpiryDate != "" {
			fmt.Printf("  Expiry Date: %s\n", record.ExpiryDate)
		}
		if record.RemainCount != "" {
			fmt.Printf("  Remain Count: %s\n", record.RemainCount)
		}
		if record.ErrorMessage != "" {
			fmt.Printf("  Error: %s\n", record.ErrorMessage)
		}
		fmt.Println()
	}
}
