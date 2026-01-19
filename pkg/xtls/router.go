// Package xtls provides Router service client for routing rules
package xtls

import (
	"context"
	"fmt"

	"github.com/xtls/xray-core/app/router"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// RouterServiceClient wraps the Xray router service
type RouterServiceClient struct {
	conn   *grpc.ClientConn
	client routerCommand.RoutingServiceClient
	logger *zap.Logger
}

// NewRouterServiceClient creates a new router service client
func NewRouterServiceClient(conn *grpc.ClientConn, logger *zap.Logger) *RouterServiceClient {
	return &RouterServiceClient{
		conn:   conn,
		client: routerCommand.NewRoutingServiceClient(conn),
		logger: logger,
	}
}

// AddRule adds a routing rule to block source IP
func (c *RouterServiceClient) AddRule(ctx context.Context, ruleTag string, ip string) error {
	if c.client == nil {
		return fmt.Errorf("router client not initialized")
	}

	// Create routing rule config
	rule := &router.RoutingRule{
		RuleTag: ruleTag,
		TargetTag: &router.RoutingRule_Tag{
			Tag: "BLOCK",
		},
		SourceGeoip: []*router.GeoIP{
			{
				Cidr: []*router.CIDR{
					{
						Ip:     net.ParseAddress(ip).IP(),
						Prefix: 32,
					},
				},
			},
		},
	}

	// Serialize to TypedMessage
	config := serial.ToTypedMessage(rule)

	req := &routerCommand.AddRuleRequest{
		ShouldAppend: true,
		Config:       config,
	}

	_, err := c.client.AddRule(ctx, req)
	if err != nil {
		c.logger.Error("Failed to add routing rule",
			zap.String("ruleTag", ruleTag),
			zap.String("ip", ip),
			zap.Error(err))
		return err
	}

	c.logger.Info("Added routing rule",
		zap.String("ruleTag", ruleTag),
		zap.String("ip", ip))

	return nil
}

// RemoveRule removes a routing rule by tag
func (c *RouterServiceClient) RemoveRule(ctx context.Context, ruleTag string, ip string) error {
	if c.client == nil {
		return fmt.Errorf("router client not initialized")
	}

	req := &routerCommand.RemoveRuleRequest{
		RuleTag: ruleTag,
	}

	_, err := c.client.RemoveRule(ctx, req)
	if err != nil {
		c.logger.Error("Failed to remove routing rule",
			zap.String("ruleTag", ruleTag),
			zap.Error(err))
		return err
	}

	c.logger.Info("Removed routing rule",
		zap.String("ruleTag", ruleTag))

	return nil
}
