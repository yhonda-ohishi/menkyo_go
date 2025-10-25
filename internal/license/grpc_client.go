package license

import (
	"context"
	"fmt"
	"time"

	pb "menkyo_go/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client gRPCクライアント
type Client struct {
	conn   *grpc.ClientConn
	client pb.LicenseReaderClient
	target string
}

// NewClient 新しいClientを作成
func NewClient(target string) (*Client, error) {
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := pb.NewLicenseReaderClient(conn)

	return &Client{
		conn:   conn,
		client: client,
		target: target,
	}, nil
}

// Close クライアントを閉じる
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PushLicenseData 免許証データをプッシュ
func (c *Client) PushLicenseData(data *pb.LicenseData) (*pb.PushResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.PushLicenseData(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to push license data: %w", err)
	}

	return resp, nil
}

// PushReadLog 読み取りログをプッシュ
func (c *Client) PushReadLog(logData *pb.ReadLog) (*pb.PushResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.PushReadLog(ctx, logData)
	if err != nil {
		return nil, fmt.Errorf("failed to push read log: %w", err)
	}

	return resp, nil
}
