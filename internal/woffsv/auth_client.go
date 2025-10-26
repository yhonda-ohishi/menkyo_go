package woffsv

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	authv1 "menkyo_go/proto/auth/v1"
	"menkyo_go/proto/auth/v1/authv1connect"
)

// AuthClient woff-svのAuthServiceクライアント (Connect RPC)
type AuthClient struct {
	client     authv1connect.AuthServiceClient
	apiSecret  string
	backendURL string
}

// NewAuthClient 新しいAuthClientを作成 (Connect RPC)
func NewAuthClient(backendURL string, apiSecret string) (*AuthClient, error) {
	// Connect RPCクライアントを作成
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := authv1connect.NewAuthServiceClient(
		httpClient,
		backendURL,
	)

	return &AuthClient{
		client:     client,
		apiSecret:  apiSecret,
		backendURL: backendURL,
	}, nil
}

// Close 接続をクローズ (Connect RPCでは不要だが互換性のため残す)
func (c *AuthClient) Close() error {
	return nil
}

// CreateTimeCard TimeCardLogを作成 (DEV環境)
func (c *AuthClient) CreateTimeCard(driverID int32, cardID string, state string, machineIP string) (*authv1.TimeCardLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	req := connect.NewRequest(&authv1.CreateTimeCardLogRequest{
		Datetime:    now.Format(time.RFC3339),
		Id:          driverID,
		CardId:      cardID, // カードIDフィールドに設定
		MachineIp:   machineIP,
		State:       state,
		StateDetail: "", // state_detailは空に
	})

	// 認証ヘッダーを追加
	req.Header().Set("x-api-secret", c.apiSecret)

	resp, err := c.client.CreateTimeCardLog(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create time card log: %w", err)
	}

	return resp.Msg.Log, nil
}
