package license

import (
	"context"
	"fmt"
	"time"

	pb "menkyo_go/proto/license"
	dbpb "github.com/yhonda-ohishi/db_service/src/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client gRPCクライアント
type Client struct {
	conn         *grpc.ClientConn
	client       pb.LicenseReaderClient
	target       string
	dbConn       *grpc.ClientConn
	dbClient     dbpb.Db_TimeCardDevServiceClient
	dbServerAddr string
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

// NewClientWithDB DBサーバー接続付きのClientを作成
func NewClientWithDB(target, dbServerAddr string) (*Client, error) {
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := pb.NewLicenseReaderClient(conn)

	// DBサーバーに接続
	dbConn, err := grpc.Dial(dbServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to db server: %w", err)
	}

	dbClient := dbpb.NewDb_TimeCardDevServiceClient(dbConn)

	return &Client{
		conn:         conn,
		client:       client,
		target:       target,
		dbConn:       dbConn,
		dbClient:     dbClient,
		dbServerAddr: dbServerAddr,
	}, nil
}

// Close クライアントを閉じる
func (c *Client) Close() error {
	var err error
	if c.dbConn != nil {
		if closeErr := c.dbConn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
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

// InsertTimeCard TimeCardをDBに挿入
func (c *Client) InsertTimeCard(driverID int32, cardID string, state string) (*dbpb.Db_TimeCardResponse, error) {
	if c.dbClient == nil {
		return nil, fmt.Errorf("db client not initialized - use NewClientWithDB")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	timeCard := &dbpb.Db_TimeCard{
		Datetime:   now.Format(time.RFC3339),
		Id:         driverID,
		MachineIp:  "",  // リーダーのIPまたはID
		State:      state,
		Created:    now.Format(time.RFC3339),
		Modified:   now.Format(time.RFC3339),
	}

	// state_detailにカードIDを設定（オプショナル）
	if cardID != "" {
		timeCard.StateDetail = &cardID
	}

	req := &dbpb.Db_CreateTimeCardRequest{
		TimeCard: timeCard,
	}

	resp, err := c.dbClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to insert time card: %w", err)
	}

	return resp, nil
}
