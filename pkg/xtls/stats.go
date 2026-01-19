// Package xtls provides Stats service client for traffic statistics
package xtls

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/xtls/xray-core/app/stats/command"
)

// StatsServiceClient wraps the Xray stats service
type StatsServiceClient struct {
	client command.StatsServiceClient
	logger *zap.Logger
}

// UserStats represents traffic statistics for a user
type UserStats struct {
	Email    string
	Uplink   int64
	Downlink int64
}

// SystemStats represents system-wide statistics (traffic based)
type SystemStats struct {
	InboundUplink    int64
	InboundDownlink  int64
	OutboundUplink   int64
	OutboundDownlink int64
}

// SysStats represents Xray's internal system stats (matches Node.js GetSystemStatsResponseModel)
type SysStats struct {
	NumGoroutine int
	NumGC        int
	Alloc        int64
	TotalAlloc   int64
	Sys          int64
	Mallocs      int64
	Frees        int64
	LiveObjects  int64
	PauseTotalNs int64
	Uptime       int64
}

// GetUserStats retrieves stats for a specific user
func (c *StatsServiceClient) GetUserStats(ctx context.Context, email string, reset bool) (*UserStats, error) {
	stats := &UserStats{Email: email}

	// Get uplink stats
	uplinkName := "user>>>" + email + ">>>traffic>>>uplink"
	uplinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   uplinkName,
		Reset_: reset,
	})
	if err != nil {
		c.logger.Debug("Failed to get uplink stats", zap.String("email", email), zap.Error(err))
	} else if uplinkResp.Stat != nil {
		stats.Uplink = uplinkResp.Stat.Value
	}

	// Get downlink stats
	downlinkName := "user>>>" + email + ">>>traffic>>>downlink"
	downlinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   downlinkName,
		Reset_: reset,
	})
	if err != nil {
		c.logger.Debug("Failed to get downlink stats", zap.String("email", email), zap.Error(err))
	} else if downlinkResp.Stat != nil {
		stats.Downlink = downlinkResp.Stat.Value
	}

	return stats, nil
}

// GetAllUserStats retrieves stats for all users
func (c *StatsServiceClient) GetAllUserStats(ctx context.Context, reset bool) ([]*UserStats, error) {
	// Query all user stats
	resp, err := c.client.QueryStats(ctx, &command.QueryStatsRequest{
		Pattern: "user>>>",
		Reset_:  reset,
	})
	if err != nil {
		return nil, err
	}

	// Parse stats into UserStats objects
	userMap := make(map[string]*UserStats)

	for _, stat := range resp.Stat {
		// Parse stat name: user>>>email>>>traffic>>>direction
		parts := strings.Split(stat.Name, ">>>")
		if len(parts) < 4 {
			continue
		}

		email := parts[1]
		direction := parts[3]

		if _, ok := userMap[email]; !ok {
			userMap[email] = &UserStats{Email: email}
		}

		switch direction {
		case "uplink":
			userMap[email].Uplink = stat.Value
		case "downlink":
			userMap[email].Downlink = stat.Value
		}
	}

	// Convert map to slice
	result := make([]*UserStats, 0, len(userMap))
	for _, stats := range userMap {
		result = append(result, stats)
	}

	c.logger.Debug("GetAllUserStats completed", zap.Int("count", len(result)))
	return result, nil
}

// GetSysStats retrieves Xray's internal system stats (matches Node.js format)
func (c *StatsServiceClient) GetSysStats(ctx context.Context) (*SysStats, error) {
	resp, err := c.client.GetSysStats(ctx, &command.SysStatsRequest{})
	if err != nil {
		c.logger.Error("Failed to get sys stats", zap.Error(err))
		return nil, err
	}

	return &SysStats{
		NumGoroutine: int(resp.NumGoroutine),
		NumGC:        int(resp.NumGC),
		Alloc:        int64(resp.Alloc),
		TotalAlloc:   int64(resp.TotalAlloc),
		Sys:          int64(resp.Sys),
		Mallocs:      int64(resp.Mallocs),
		Frees:        int64(resp.Frees),
		LiveObjects:  int64(resp.LiveObjects),
		PauseTotalNs: int64(resp.PauseTotalNs),
		Uptime:       int64(resp.Uptime),
	}, nil
}

// GetSystemStats retrieves system-wide statistics (traffic based)
func (c *StatsServiceClient) GetSystemStats(ctx context.Context, reset bool) (*SystemStats, error) {
	stats := &SystemStats{}

	// Get inbound stats
	inboundResp, err := c.client.QueryStats(ctx, &command.QueryStatsRequest{
		Pattern: "inbound>>>",
		Reset_:  reset,
	})
	if err == nil {
		for _, stat := range inboundResp.Stat {
			if strings.HasSuffix(stat.Name, ">>>uplink") {
				stats.InboundUplink += stat.Value
			} else if strings.HasSuffix(stat.Name, ">>>downlink") {
				stats.InboundDownlink += stat.Value
			}
		}
	}

	// Get outbound stats
	outboundResp, err := c.client.QueryStats(ctx, &command.QueryStatsRequest{
		Pattern: "outbound>>>",
		Reset_:  reset,
	})
	if err == nil {
		for _, stat := range outboundResp.Stat {
			if strings.HasSuffix(stat.Name, ">>>uplink") {
				stats.OutboundUplink += stat.Value
			} else if strings.HasSuffix(stat.Name, ">>>downlink") {
				stats.OutboundDownlink += stat.Value
			}
		}
	}

	return stats, nil
}

// GetInboundStats retrieves stats for a specific inbound tag
func (c *StatsServiceClient) GetInboundStats(ctx context.Context, tag string, reset bool) (uplink, downlink int64, err error) {
	// Get uplink
	uplinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   "inbound>>>" + tag + ">>>traffic>>>uplink",
		Reset_: reset,
	})
	if err == nil && uplinkResp.Stat != nil {
		uplink = uplinkResp.Stat.Value
	}

	// Get downlink
	downlinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   "inbound>>>" + tag + ">>>traffic>>>downlink",
		Reset_: reset,
	})
	if err == nil && downlinkResp.Stat != nil {
		downlink = downlinkResp.Stat.Value
	}

	return uplink, downlink, nil
}

// GetOutboundStats retrieves stats for a specific outbound tag
func (c *StatsServiceClient) GetOutboundStats(ctx context.Context, tag string, reset bool) (uplink, downlink int64, err error) {
	// Get uplink
	uplinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   "outbound>>>" + tag + ">>>traffic>>>uplink",
		Reset_: reset,
	})
	if err == nil && uplinkResp.Stat != nil {
		uplink = uplinkResp.Stat.Value
	}

	// Get downlink
	downlinkResp, err := c.client.GetStats(ctx, &command.GetStatsRequest{
		Name:   "outbound>>>" + tag + ">>>traffic>>>downlink",
		Reset_: reset,
	})
	if err == nil && downlinkResp.Stat != nil {
		downlink = downlinkResp.Stat.Value
	}

	return uplink, downlink, nil
}

// GetUserOnlineStatus checks if a user is currently online using GetStatsOnline
func (c *StatsServiceClient) GetUserOnlineStatus(ctx context.Context, email string) (bool, error) {
	// Use GetStatsOnline to check if user has active connections
	resp, err := c.client.GetStatsOnline(ctx, &command.GetStatsRequest{
		Name: "user>>>" + email + ">>>traffic>>>uplink",
	})
	if err != nil {
		c.logger.Debug("Failed to get online status", zap.String("email", email), zap.Error(err))
		return false, nil
	}

	// User is online if there's any value returned
	return resp.Stat != nil && resp.Stat.Value > 0, nil
}

// InboundTraffic represents traffic stats for an inbound
type InboundTraffic struct {
	Tag      string `json:"tag"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// OutboundTraffic represents traffic stats for an outbound
type OutboundTraffic struct {
	Tag      string `json:"tag"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetAllInboundsStats retrieves stats for all inbounds
func (c *StatsServiceClient) GetAllInboundsStats(ctx context.Context, reset bool) ([]*InboundTraffic, error) {
	resp, err := c.client.QueryStats(ctx, &command.QueryStatsRequest{
		Pattern: "inbound>>>",
		Reset_:  reset,
	})
	if err != nil {
		return nil, err
	}

	// Parse stats into InboundTraffic objects
	inboundMap := make(map[string]*InboundTraffic)

	for _, stat := range resp.Stat {
		// Parse stat name: inbound>>>tag>>>traffic>>>direction
		parts := strings.Split(stat.Name, ">>>")
		if len(parts) < 4 {
			continue
		}

		tag := parts[1]
		direction := parts[3]

		if _, ok := inboundMap[tag]; !ok {
			inboundMap[tag] = &InboundTraffic{Tag: tag}
		}

		switch direction {
		case "uplink":
			inboundMap[tag].Uplink = stat.Value
		case "downlink":
			inboundMap[tag].Downlink = stat.Value
		}
	}

	// Convert map to slice
	result := make([]*InboundTraffic, 0, len(inboundMap))
	for _, traffic := range inboundMap {
		result = append(result, traffic)
	}

	c.logger.Debug("GetAllInboundsStats completed", zap.Int("count", len(result)))
	return result, nil
}

// GetAllOutboundsStats retrieves stats for all outbounds
func (c *StatsServiceClient) GetAllOutboundsStats(ctx context.Context, reset bool) ([]*OutboundTraffic, error) {
	resp, err := c.client.QueryStats(ctx, &command.QueryStatsRequest{
		Pattern: "outbound>>>",
		Reset_:  reset,
	})
	if err != nil {
		return nil, err
	}

	// Parse stats into OutboundTraffic objects
	outboundMap := make(map[string]*OutboundTraffic)

	for _, stat := range resp.Stat {
		// Parse stat name: outbound>>>tag>>>traffic>>>direction
		parts := strings.Split(stat.Name, ">>>")
		if len(parts) < 4 {
			continue
		}

		tag := parts[1]
		direction := parts[3]

		if _, ok := outboundMap[tag]; !ok {
			outboundMap[tag] = &OutboundTraffic{Tag: tag}
		}

		switch direction {
		case "uplink":
			outboundMap[tag].Uplink = stat.Value
		case "downlink":
			outboundMap[tag].Downlink = stat.Value
		}
	}

	// Convert map to slice
	result := make([]*OutboundTraffic, 0, len(outboundMap))
	for _, traffic := range outboundMap {
		result = append(result, traffic)
	}

	c.logger.Debug("GetAllOutboundsStats completed", zap.Int("count", len(result)))
	return result, nil
}
