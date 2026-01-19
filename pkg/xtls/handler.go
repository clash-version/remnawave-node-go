// Package xtls provides Handler service client for user management
package xtls

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/protobuf/proto"
)

// Protocol types for proxy configuration
const (
	ProtocolVLESS       = "vless"
	ProtocolTrojan      = "trojan"
	ProtocolShadowsocks = "shadowsocks"
)

// HandlerServiceClient wraps the Xray handler service
type HandlerServiceClient struct {
	client command.HandlerServiceClient
	logger *zap.Logger
}

// UserAccount represents a user account configuration
type UserAccount struct {
	Email    string
	UUID     string // For VLESS
	Password string // For Trojan/Shadowsocks
	Level    uint32
	Flow     string // For VLESS flow
	Method   string // For Shadowsocks cipher
}

// AddUserRequest represents a request to add a user
type AddUserRequest struct {
	Tag      string
	Protocol string
	Account  *UserAccount
}

// RemoveUserRequest represents a request to remove a user
type RemoveUserRequest struct {
	Tag   string
	Email string
}

// AddUser adds a user to the specified inbound tag
func (c *HandlerServiceClient) AddUser(ctx context.Context, req *AddUserRequest) error {
	// Create the account based on protocol type
	var account proto.Message

	switch req.Protocol {
	case ProtocolVLESS:
		account = &vless.Account{
			Id:   req.Account.UUID,
			Flow: req.Account.Flow,
		}
	case ProtocolTrojan:
		account = &trojan.Account{
			Password: req.Account.Password,
		}
	case ProtocolShadowsocks:
		account = &shadowsocks.Account{
			Password:   req.Account.Password,
			CipherType: getCipherType(req.Account.Method),
		}
	default:
		return fmt.Errorf("unsupported protocol: %s", req.Protocol)
	}

	// Create the user
	user := &protocol.User{
		Level:   req.Account.Level,
		Email:   req.Account.Email,
		Account: serial.ToTypedMessage(account),
	}

	// Create add user operation
	addUserOp := &command.AddUserOperation{
		User: user,
	}

	opTyped := serial.ToTypedMessage(addUserOp)

	// Call the gRPC service
	_, err := c.client.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       req.Tag,
		Operation: opTyped,
	})

	if err != nil {
		c.logger.Error("Failed to add user via gRPC",
			zap.String("email", req.Account.Email),
			zap.String("tag", req.Tag),
			zap.Error(err))
		return fmt.Errorf("failed to add user: %w", err)
	}

	c.logger.Info("Added user",
		zap.String("email", req.Account.Email),
		zap.String("tag", req.Tag),
		zap.String("protocol", req.Protocol))

	return nil
}

// RemoveUser removes a user from the specified inbound tag
func (c *HandlerServiceClient) RemoveUser(ctx context.Context, req *RemoveUserRequest) error {
	// Create remove user operation
	removeUserOp := &command.RemoveUserOperation{
		Email: req.Email,
	}

	opTyped := serial.ToTypedMessage(removeUserOp)

	// Call the gRPC service
	_, err := c.client.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       req.Tag,
		Operation: opTyped,
	})

	if err != nil {
		c.logger.Error("Failed to remove user via gRPC",
			zap.String("email", req.Email),
			zap.String("tag", req.Tag),
			zap.Error(err))
		return fmt.Errorf("failed to remove user: %w", err)
	}

	c.logger.Info("Removed user",
		zap.String("email", req.Email),
		zap.String("tag", req.Tag))

	return nil
}

// getCipherType converts cipher string to Shadowsocks cipher type
func getCipherType(method string) shadowsocks.CipherType {
	switch method {
	case "aes-128-gcm":
		return shadowsocks.CipherType_AES_128_GCM
	case "aes-256-gcm":
		return shadowsocks.CipherType_AES_256_GCM
	case "chacha20-poly1305", "chacha20-ietf-poly1305":
		return shadowsocks.CipherType_CHACHA20_POLY1305
	case "2022-blake3-aes-128-gcm":
		return shadowsocks.CipherType_AES_128_GCM
	case "2022-blake3-aes-256-gcm":
		return shadowsocks.CipherType_AES_256_GCM
	case "2022-blake3-chacha20-poly1305":
		return shadowsocks.CipherType_CHACHA20_POLY1305
	default:
		return shadowsocks.CipherType_AES_128_GCM
	}
}

// InboundUser represents a user in an inbound (matches Node.js IInboundUser)
type InboundUser struct {
	Username string `json:"username"`
	Level    uint32 `json:"level"`
	Protocol string `json:"protocol"`
}

// GetInboundUsersResponse represents the response for GetInboundUsers
type GetInboundUsersResponse struct {
	Users []InboundUser
}

// getProtocolFromAccount extracts protocol name from account type
func getProtocolFromAccount(account *serial.TypedMessage) string {
	if account == nil {
		return "unknown"
	}
	// The Type field contains the full type URL like "xray.proxy.vless.Account"
	typeURL := account.Type
	switch {
	case typeURL == "xray.proxy.trojan.Account":
		return "trojan"
	case typeURL == "xray.proxy.vless.Account":
		return "vless"
	case typeURL == "xray.proxy.shadowsocks.Account":
		return "shadowsocks"
	case typeURL == "xray.proxy.shadowsocks_2022.Account":
		return "shadowsocks2022"
	case typeURL == "xray.proxy.socks.Account":
		return "socks"
	case typeURL == "xray.proxy.http.Account":
		return "http"
	default:
		return "unknown"
	}
}

// GetInboundUsers returns the list of users in the specified inbound tag
func (c *HandlerServiceClient) GetInboundUsers(ctx context.Context, tag string) (*GetInboundUsersResponse, error) {
	resp, err := c.client.GetInboundUsers(ctx, &command.GetInboundUserRequest{
		Tag: tag,
	})
	if err != nil {
		c.logger.Error("Failed to get inbound users via gRPC",
			zap.String("tag", tag),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get inbound users: %w", err)
	}

	users := make([]InboundUser, len(resp.Users))
	for i, user := range resp.Users {
		users[i] = InboundUser{
			Username: user.Email,
			Level:    user.Level,
			Protocol: getProtocolFromAccount(user.Account),
		}
	}

	c.logger.Debug("Got inbound users",
		zap.String("tag", tag),
		zap.Int("count", len(users)))

	return &GetInboundUsersResponse{Users: users}, nil
}

// GetInboundUsersCount returns the count of users in the specified inbound tag
func (c *HandlerServiceClient) GetInboundUsersCount(ctx context.Context, tag string) (int64, error) {
	resp, err := c.client.GetInboundUsersCount(ctx, &command.GetInboundUserRequest{
		Tag: tag,
	})
	if err != nil {
		c.logger.Error("Failed to get inbound users count via gRPC",
			zap.String("tag", tag),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get inbound users count: %w", err)
	}

	c.logger.Debug("Got inbound users count",
		zap.String("tag", tag),
		zap.Int64("count", resp.Count))

	return resp.Count, nil
}
