// Package xtls provides a gRPC client for communicating with Xray-core
package xtls

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	handlerService "github.com/xtls/xray-core/app/proxyman/command"
	statsService "github.com/xtls/xray-core/app/stats/command"
)

// Client represents a gRPC client for Xray-core
type Client struct {
	conn   *grpc.ClientConn
	addr   string
	mu     sync.RWMutex
	logger *zap.Logger

	// gRPC service clients
	handlerClient handlerService.HandlerServiceClient
	statsClient   statsService.StatsServiceClient
}

// Config holds the configuration for the Xray gRPC client
type Config struct {
	IP   string
	Port int
}

// NewClient creates a new Xray gRPC client
func NewClient(cfg *Config, logger *zap.Logger) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)

	client := &Client{
		addr:   addr,
		logger: logger,
	}

	return client, nil
}

// Connect establishes connection to Xray-core gRPC server
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	c.logger.Info("Connecting to Xray gRPC server", zap.String("addr", c.addr))

	// Create connection with timeout
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to Xray gRPC: %w", err)
	}

	c.conn = conn
	c.handlerClient = handlerService.NewHandlerServiceClient(conn)
	c.statsClient = statsService.NewStatsServiceClient(conn)

	c.logger.Info("Connected to Xray gRPC server")
	return nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.handlerClient = nil
		c.statsClient = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected to Xray-core
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

// WaitForReady waits for the Xray gRPC server to be ready
func (c *Client) WaitForReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Xray gRPC server: %w", ctx.Err())
		case <-ticker.C:
			err := c.Connect(ctx)
			if err == nil {
				return nil
			}
			c.logger.Debug("Waiting for Xray gRPC server", zap.Error(err))
		}
	}
}

// Handler returns the handler service wrapper
func (c *Client) Handler() *HandlerServiceClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.handlerClient == nil {
		return nil
	}
	return &HandlerServiceClient{client: c.handlerClient, logger: c.logger}
}

// Stats returns the stats service wrapper
func (c *Client) Stats() *StatsServiceClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.statsClient == nil {
		return nil
	}
	return &StatsServiceClient{client: c.statsClient, logger: c.logger}
}

// Router returns a router service wrapper (placeholder for now)
func (c *Client) Router() *RouterServiceClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return nil
	}
	return &RouterServiceClient{conn: c.conn, logger: c.logger}
}
