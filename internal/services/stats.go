// Package services provides business logic for statistics collection
package services

import (
	"context"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/clash-version/remnawave-node-go/pkg/xtls"
)

// StatsService manages traffic statistics
type StatsService struct {
	mu     sync.RWMutex
	logger *zap.Logger
	xtls   *xtls.Client
}

// NewStatsService creates a new StatsService
func NewStatsService(xtls *xtls.Client, logger *zap.Logger) *StatsService {
	return &StatsService{
		logger: logger,
		xtls:   xtls,
	}
}

// UserTraffic represents traffic data for a user
// Matches Node.js IUserStat: { username: string, uplink: number, downlink: number }
type UserTraffic struct {
	Username string `json:"username"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetUserStatsRequest represents a request to get user stats
type GetUserStatsRequest struct {
	Email string `json:"email"`
	Reset bool   `json:"reset"`
}

// GetUserStatsResponse represents user statistics
type GetUserStatsResponse struct {
	Email    string `json:"email"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetUserStats gets traffic statistics for a specific user
func (s *StatsService) GetUserStats(ctx context.Context, req *GetUserStatsRequest) (*GetUserStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return nil, nil
	}

	userStats, err := stats.GetUserStats(ctx, req.Email, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get user stats",
			zap.String("email", req.Email),
			zap.Error(err))
		return nil, err
	}

	return &GetUserStatsResponse{
		Email:    userStats.Email,
		Uplink:   userStats.Uplink,
		Downlink: userStats.Downlink,
	}, nil
}

// GetAllUsersStatsRequest represents a request to get all users stats
type GetAllUsersStatsRequest struct {
	Reset bool `json:"reset"`
}

// GetAllUsersStatsResponse represents all users statistics
type GetAllUsersStatsResponse struct {
	Users []*UserTraffic `json:"users"`
}

// GetAllUsersStats gets traffic statistics for all users
// Always filters out users with zero traffic (matches Node.js behavior)
func (s *StatsService) GetAllUsersStats(ctx context.Context, req *GetAllUsersStatsRequest) (*GetAllUsersStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetAllUsersStatsResponse{Users: []*UserTraffic{}}, nil
	}

	allStats, err := stats.GetAllUserStats(ctx, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get all user stats", zap.Error(err))
		return nil, err
	}

	users := make([]*UserTraffic, 0, len(allStats))
	for _, stat := range allStats {
		// Always filter out users with zero traffic (matches Node.js)
		if stat.Uplink == 0 && stat.Downlink == 0 {
			continue
		}
		users = append(users, &UserTraffic{
			Username: stat.Email,
			Uplink:   stat.Uplink,
			Downlink: stat.Downlink,
		})
	}

	return &GetAllUsersStatsResponse{Users: users}, nil
}

// SystemStatsResponse represents system statistics
// Matches Node.js GetSystemStatsResponseModel from xtls-sdk
type SystemStatsResponse struct {
	NumGoroutine int   `json:"numGoroutine"`
	NumGC        int   `json:"numGC"`
	Alloc        int64 `json:"alloc"`
	TotalAlloc   int64 `json:"totalAlloc"`
	Sys          int64 `json:"sys"`
	Mallocs      int64 `json:"mallocs"`
	Frees        int64 `json:"frees"`
	LiveObjects  int64 `json:"liveObjects"`
	PauseTotalNs int64 `json:"pauseTotalNs"`
	Uptime       int64 `json:"uptime"`
}

var startTime = time.Now()

// GetSystemStats gets system-wide statistics (matches Node.js GetSystemStatsResponseModel)
func (s *StatsService) GetSystemStats(ctx context.Context) (*SystemStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		// Fallback to local Go stats
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		return &SystemStatsResponse{
			NumGoroutine: runtime.NumGoroutine(),
			NumGC:        int(memStats.NumGC),
			Alloc:        int64(memStats.Alloc),
			TotalAlloc:   int64(memStats.TotalAlloc),
			Sys:          int64(memStats.Sys),
			Mallocs:      int64(memStats.Mallocs),
			Frees:        int64(memStats.Frees),
			LiveObjects:  int64(memStats.Mallocs - memStats.Frees),
			PauseTotalNs: int64(memStats.PauseTotalNs),
			Uptime:       int64(time.Since(startTime).Seconds()),
		}, nil
	}

	// Get Xray's internal system stats
	sysStats, err := stats.GetSysStats(ctx)
	if err != nil {
		s.logger.Warn("Failed to get Xray sys stats", zap.Error(err))
		// Fallback to local Go stats
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		return &SystemStatsResponse{
			NumGoroutine: runtime.NumGoroutine(),
			NumGC:        int(memStats.NumGC),
			Alloc:        int64(memStats.Alloc),
			TotalAlloc:   int64(memStats.TotalAlloc),
			Sys:          int64(memStats.Sys),
			Mallocs:      int64(memStats.Mallocs),
			Frees:        int64(memStats.Frees),
			LiveObjects:  int64(memStats.Mallocs - memStats.Frees),
			PauseTotalNs: int64(memStats.PauseTotalNs),
			Uptime:       int64(time.Since(startTime).Seconds()),
		}, nil
	}

	return &SystemStatsResponse{
		NumGoroutine: sysStats.NumGoroutine,
		NumGC:        sysStats.NumGC,
		Alloc:        sysStats.Alloc,
		TotalAlloc:   sysStats.TotalAlloc,
		Sys:          sysStats.Sys,
		Mallocs:      sysStats.Mallocs,
		Frees:        sysStats.Frees,
		LiveObjects:  sysStats.LiveObjects,
		PauseTotalNs: sysStats.PauseTotalNs,
		Uptime:       sysStats.Uptime,
	}, nil
}

// GetUsersStatsAndResetRequest represents request to get and reset stats
type GetUsersStatsAndResetRequest struct {
	Emails []string `json:"emails"`
}

// GetUsersStatsAndResetResponse represents response with reset stats
type GetUsersStatsAndResetResponse struct {
	Users []*UserTraffic `json:"users"`
}

// GetUsersStatsAndReset gets traffic for specific users and resets counters
func (s *StatsService) GetUsersStatsAndReset(ctx context.Context, req *GetUsersStatsAndResetRequest) (*GetUsersStatsAndResetResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetUsersStatsAndResetResponse{Users: []*UserTraffic{}}, nil
	}

	users := make([]*UserTraffic, 0, len(req.Emails))
	for _, email := range req.Emails {
		userStats, err := stats.GetUserStats(ctx, email, true)
		if err != nil {
			s.logger.Debug("Failed to get stats for user",
				zap.String("email", email),
				zap.Error(err))
			continue
		}
		users = append(users, &UserTraffic{
			Username: userStats.Email,
			Uplink:   userStats.Uplink,
			Downlink: userStats.Downlink,
		})
	}

	return &GetUsersStatsAndResetResponse{Users: users}, nil
}

// GetUserOnlineStatusRequest represents request to get user online status
type GetUserOnlineStatusRequest struct {
	Email string `json:"email"`
}

// GetUserOnlineStatusResponse represents response for user online status
// Matches Node.js GetUserOnlineStatusResponseModel: { isOnline: boolean }
type GetUserOnlineStatusResponse struct {
	IsOnline bool `json:"isOnline"`
}

// GetUserOnlineStatus checks if a user is currently online
func (s *StatsService) GetUserOnlineStatus(ctx context.Context, req *GetUserOnlineStatusRequest) (*GetUserOnlineStatusResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetUserOnlineStatusResponse{IsOnline: false}, nil
	}

	online, err := stats.GetUserOnlineStatus(ctx, req.Email)
	if err != nil {
		s.logger.Debug("Failed to get user online status",
			zap.String("email", req.Email),
			zap.Error(err))
		return &GetUserOnlineStatusResponse{IsOnline: false}, nil
	}

	return &GetUserOnlineStatusResponse{IsOnline: online}, nil
}

// InboundStats represents traffic stats for an inbound
type InboundStats struct {
	Inbound  string `json:"inbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetInboundStatsRequest represents request to get inbound stats
type GetInboundStatsRequest struct {
	Tag   string `json:"tag"`
	Reset bool   `json:"reset"`
}

// GetInboundStatsResponse represents response for inbound stats
type GetInboundStatsResponse struct {
	Inbound  string `json:"inbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetInboundStats gets traffic statistics for a specific inbound
func (s *StatsService) GetInboundStats(ctx context.Context, req *GetInboundStatsRequest) (*GetInboundStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetInboundStatsResponse{Inbound: req.Tag}, nil
	}

	uplink, downlink, err := stats.GetInboundStats(ctx, req.Tag, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get inbound stats",
			zap.String("tag", req.Tag),
			zap.Error(err))
		return nil, err
	}

	return &GetInboundStatsResponse{
		Inbound:  req.Tag,
		Uplink:   uplink,
		Downlink: downlink,
	}, nil
}

// OutboundStats represents traffic stats for an outbound
type OutboundStats struct {
	Outbound string `json:"outbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetOutboundStatsRequest represents request to get outbound stats
type GetOutboundStatsRequest struct {
	Tag   string `json:"tag"`
	Reset bool   `json:"reset"`
}

// GetOutboundStatsResponse represents response for outbound stats
type GetOutboundStatsResponse struct {
	Outbound string `json:"outbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

// GetOutboundStats gets traffic statistics for a specific outbound
func (s *StatsService) GetOutboundStats(ctx context.Context, req *GetOutboundStatsRequest) (*GetOutboundStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetOutboundStatsResponse{Outbound: req.Tag}, nil
	}

	uplink, downlink, err := stats.GetOutboundStats(ctx, req.Tag, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get outbound stats",
			zap.String("tag", req.Tag),
			zap.Error(err))
		return nil, err
	}

	return &GetOutboundStatsResponse{
		Outbound: req.Tag,
		Uplink:   uplink,
		Downlink: downlink,
	}, nil
}

// GetAllInboundsStatsRequest represents request to get all inbounds stats
type GetAllInboundsStatsRequest struct {
	Reset bool `json:"reset"`
}

// GetAllInboundsStatsResponse represents response for all inbounds stats
type GetAllInboundsStatsResponse struct {
	Inbounds []*InboundStats `json:"inbounds"`
}

// GetAllInboundsStats gets traffic statistics for all inbounds
func (s *StatsService) GetAllInboundsStats(ctx context.Context, req *GetAllInboundsStatsRequest) (*GetAllInboundsStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetAllInboundsStatsResponse{Inbounds: []*InboundStats{}}, nil
	}

	inbounds, err := stats.GetAllInboundsStats(ctx, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get all inbounds stats", zap.Error(err))
		return nil, err
	}

	result := make([]*InboundStats, 0, len(inbounds))
	for _, inbound := range inbounds {
		result = append(result, &InboundStats{
			Inbound:  inbound.Tag,
			Uplink:   inbound.Uplink,
			Downlink: inbound.Downlink,
		})
	}

	return &GetAllInboundsStatsResponse{Inbounds: result}, nil
}

// GetAllOutboundsStatsRequest represents request to get all outbounds stats
type GetAllOutboundsStatsRequest struct {
	Reset bool `json:"reset"`
}

// GetAllOutboundsStatsResponse represents response for all outbounds stats
type GetAllOutboundsStatsResponse struct {
	Outbounds []*OutboundStats `json:"outbounds"`
}

// GetAllOutboundsStats gets traffic statistics for all outbounds
func (s *StatsService) GetAllOutboundsStats(ctx context.Context, req *GetAllOutboundsStatsRequest) (*GetAllOutboundsStatsResponse, error) {
	stats := s.xtls.Stats()
	if stats == nil {
		return &GetAllOutboundsStatsResponse{Outbounds: []*OutboundStats{}}, nil
	}

	outbounds, err := stats.GetAllOutboundsStats(ctx, req.Reset)
	if err != nil {
		s.logger.Warn("Failed to get all outbounds stats", zap.Error(err))
		return nil, err
	}

	result := make([]*OutboundStats, 0, len(outbounds))
	for _, outbound := range outbounds {
		result = append(result, &OutboundStats{
			Outbound: outbound.Tag,
			Uplink:   outbound.Uplink,
			Downlink: outbound.Downlink,
		})
	}

	return &GetAllOutboundsStatsResponse{Outbounds: result}, nil
}

// GetCombinedStatsRequest represents request to get combined stats
type GetCombinedStatsRequest struct {
	Reset bool `json:"reset"`
}

// GetCombinedStatsResponse represents response for combined stats
type GetCombinedStatsResponse struct {
	Inbounds  []*InboundStats  `json:"inbounds"`
	Outbounds []*OutboundStats `json:"outbounds"`
}

// GetCombinedStats gets traffic statistics for all inbounds and outbounds
func (s *StatsService) GetCombinedStats(ctx context.Context, req *GetCombinedStatsRequest) (*GetCombinedStatsResponse, error) {
	inboundsResp, err := s.GetAllInboundsStats(ctx, &GetAllInboundsStatsRequest{Reset: req.Reset})
	if err != nil {
		return nil, err
	}

	outboundsResp, err := s.GetAllOutboundsStats(ctx, &GetAllOutboundsStatsRequest{Reset: req.Reset})
	if err != nil {
		return nil, err
	}

	return &GetCombinedStatsResponse{
		Inbounds:  inboundsResp.Inbounds,
		Outbounds: outboundsResp.Outbounds,
	}, nil
}
