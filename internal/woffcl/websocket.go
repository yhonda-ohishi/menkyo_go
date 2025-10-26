package woffcl

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// BackendMessage WebSocketメッセージ
type BackendMessage struct {
	Type string `json:"type"` // "waiting", "backend-connected"
	URL  string `json:"url"`  // バックエンドURL
}

// BackendWatcher バックエンドURL変更を監視
type BackendWatcher struct {
	endpoint        string
	secret          string
	onConnect       func(url string) bool // URL変更時のコールバック（成功時true、失敗時false）
	logger          func(string)
	stopChan        chan struct{}
	lastConnectedURL string // 最後に接続成功したURL
}

// NewBackendWatcher 新しいBackendWatcherを作成
func NewBackendWatcher(endpoint, secret string, onConnect func(string) bool, logger func(string)) *BackendWatcher {
	return &BackendWatcher{
		endpoint:  endpoint,
		secret:    secret,
		onConnect: onConnect,
		logger:    logger,
		stopChan:  make(chan struct{}),
	}
}

// Start WebSocket接続を開始してバックエンドURL変更を監視
func (w *BackendWatcher) Start() {
	go w.watchBackend()
}

// Stop WebSocket監視を停止
func (w *BackendWatcher) Stop() {
	close(w.stopChan)
}

func (w *BackendWatcher) log(msg string) {
	if w.logger != nil {
		w.logger(msg)
	} else {
		log.Println(msg)
	}
}

func (w *BackendWatcher) watchBackend() {
	for {
		select {
		case <-w.stopChan:
			w.log("Backend watcher stopped")
			return
		default:
			shouldReconnect := w.connectAndWatch()
			if !shouldReconnect {
				w.log("Backend watcher finished")
				return
			}
			// 切断後、5秒待ってから再接続
			select {
			case <-w.stopChan:
				return
			case <-time.After(5 * time.Second):
				w.log("Reconnecting to backend watcher...")
			}
		}
	}
}

func (w *BackendWatcher) connectAndWatch() bool {
	// WebSocket URLを構築
	u, err := url.Parse(w.endpoint)
	if err != nil {
		w.log(fmt.Sprintf("Failed to parse endpoint: %v", err))
		return true // エラーでも再接続を試みる
	}

	// HTTPをWSに、HTTPSをWSSに変換
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}

	u.Path = "/wait-for-backend"
	q := u.Query()
	q.Set("secret", w.secret)
	u.RawQuery = q.Encode()

	w.log(fmt.Sprintf("Connecting to backend watcher: %s", u.String()))

	// WebSocket接続
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		w.log(fmt.Sprintf("WebSocket dial failed: %v", err))
		return true // 再接続を試みる
	}
	defer conn.Close()

	w.log("WebSocket connected, waiting for backend changes...")

	// メッセージを受信
	for {
		select {
		case <-w.stopChan:
			return false // 停止要求
		default:
			var msg BackendMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				w.log(fmt.Sprintf("WebSocket read error: %v", err))
				return true // 再接続を試みる
			}

			w.log(fmt.Sprintf("Received message: type=%s, url=%s", msg.Type, msg.URL))

			switch msg.Type {
			case "waiting":
				w.log("Waiting for backend to connect...")
			case "backend-connected":
				// 同じURLの場合はスキップ（重複通知を避ける）
				if msg.URL == w.lastConnectedURL {
					w.log(fmt.Sprintf("Backend URL unchanged: %s", msg.URL))
					continue
				}

				w.log(fmt.Sprintf("Backend URL received: %s", msg.URL))

				// コールバックでHeartbeatチェック
				if w.onConnect != nil {
					success := w.onConnect(msg.URL)
					if success {
						w.lastConnectedURL = msg.URL
						w.log(fmt.Sprintf("Backend connected successfully: %s", msg.URL))
					} else {
						w.log(fmt.Sprintf("Backend connection failed, continuing to wait..."))
					}
				}
				// バックエンドが接続されても監視を継続
				// （次回の再起動時に新しいURLを受信するため）
			}
		}
	}
}
