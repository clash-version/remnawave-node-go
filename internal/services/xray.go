// Package services provides business logic services
package services

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/clash-version/remnawave-node-go/pkg/supervisord"
	"github.com/clash-version/remnawave-node-go/pkg/xtls"
)

// ErrXrayAlreadyProcessing indicates Xray is already being started/restarted
var ErrXrayAlreadyProcessing = errors.New("Xray is already being processed")

// XrayService manages Xray-core process and configuration
type XrayService struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	supervisor   *supervisord.Client
	xtls         *xtls.Client
	internal     *InternalService
	processName  string
	configDir    string
	xrayBinary   string
	isConfigured bool

	// Concurrency protection
	isStartProcessing atomic.Bool

	// Online status tracking
	isXrayOnline bool

	// Cached version info
	cachedVersion string

	// Disable hash check (skip restart optimization)
	disableHashedSetCheck bool
}

// XrayConfig holds Xray service configuration
type XrayConfig struct {
	ProcessName           string
	ConfigDir             string
	XrayBinary            string // Path to xray binary, default: "xray"
	DisableHashedSetCheck bool   // If true, skip hash-based restart optimization
}

// NewXrayService creates a new XrayService
func NewXrayService(cfg *XrayConfig, supervisor *supervisord.Client, xtls *xtls.Client, internal *InternalService, logger *zap.Logger) *XrayService {
	xrayBinary := cfg.XrayBinary
	if xrayBinary == "" {
		xrayBinary = "xray"
	}
	return &XrayService{
		logger:                logger,
		supervisor:            supervisor,
		xtls:                  xtls,
		internal:              internal,
		processName:           cfg.ProcessName,
		configDir:             cfg.ConfigDir,
		xrayBinary:            xrayBinary,
		isXrayOnline:          false,
		disableHashedSetCheck: cfg.DisableHashedSetCheck,
	}
}

// checkXrayHealth checks if Xray is responding to gRPC calls
func (s *XrayService) checkXrayHealth(ctx context.Context) bool {
	const maxRetries = 10
	const retryInterval = 2 * time.Second

	stats := s.xtls.Stats()
	if stats == nil {
		return false
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := stats.GetSystemStats(ctx, false)
		if err == nil {
			return true
		}

		retriesLeft := maxRetries - attempt
		if retriesLeft > 0 {
			s.logger.Debug("Get Xray internal status attempt failed",
				zap.Int("attempt", attempt),
				zap.Int("retriesLeft", retriesLeft),
				zap.Error(err))

			select {
			case <-ctx.Done():
				s.logger.Error("Context cancelled while checking Xray health")
				return false
			case <-time.After(retryInterval):
				// Continue to next attempt
			}
		}
	}

	s.logger.Error("Failed to get Xray internal status after all retries",
		zap.Int("totalAttempts", maxRetries))
	return false
}

// XrayConfigData represents the Xray configuration file structure
type XrayConfigData struct {
	Log       interface{}   `json:"log,omitempty"`
	API       interface{}   `json:"api,omitempty"`
	Inbounds  []interface{} `json:"inbounds,omitempty"`
	Outbounds []interface{} `json:"outbounds,omitempty"`
	Routing   interface{}   `json:"routing,omitempty"`
	Stats     interface{}   `json:"stats,omitempty"`
	Policy    interface{}   `json:"policy,omitempty"`
}

// Default Xray API configuration (matches Node.js XRAY_DEFAULT_API_MODEL)
var defaultAPIConfig = map[string]interface{}{
	"services": []string{"HandlerService", "StatsService", "RoutingService"},
	"listen":   "127.0.0.1:61000",
	"tag":      "REMNAWAVE_API",
}

// Default policy configuration (matches Node.js XRAY_DEFAULT_POLICY_MODEL)
var defaultPolicyConfig = map[string]interface{}{
	"levels": map[string]interface{}{
		"0": map[string]interface{}{
			"statsUserUplink":   true,
			"statsUserDownlink": true,
			"statsUserOnline":   true,
		},
	},
	"system": map[string]interface{}{
		"statsInboundDownlink":  true,
		"statsInboundUplink":    true,
		"statsOutboundDownlink": true,
		"statsOutboundUplink":   true,
	},
}

// generateApiConfig adds default API, Stats, and Policy configurations to the Xray config
// This matches the Node.js generateApiConfig function behavior
func generateApiConfig(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all existing config
	for k, v := range config {
		result[k] = v
	}

	// Add stats configuration (empty object)
	result["stats"] = map[string]interface{}{}

	// Add API configuration
	result["api"] = defaultAPIConfig

	// Build and add policy configuration
	result["policy"] = defaultPolicyConfig

	return result
}

// StartRequestInternals represents the internals part of start request (Node.js format)
type StartRequestInternals struct {
	ForceRestart bool           `json:"forceRestart"`
	Hashes       *InboundHashes `json:"hashes"`
}

// StartRequest represents a request to start Xray (Node.js compatible format)
// Format: { internals: { forceRestart, hashes }, xrayConfig: {...} }
type StartRequest struct {
	Internals  StartRequestInternals  `json:"internals"`
	XrayConfig map[string]interface{} `json:"xrayConfig"`
}

// SystemInformation represents system info in response
type SystemInformation struct {
	CPUCores    int    `json:"cpuCores"`
	CPUModel    string `json:"cpuModel"`
	MemoryTotal string `json:"memoryTotal"`
}

// NodeInformation represents node version info
type NodeInformation struct {
	Version string `json:"version"`
}

// StartResponseData represents the response data for start request (Node.js format)
type StartResponseData struct {
	IsStarted         bool               `json:"isStarted"`
	Version           *string            `json:"version"`
	Error             *string            `json:"error"`
	SystemInformation *SystemInformation `json:"systemInformation"`
	NodeInformation   NodeInformation    `json:"nodeInformation"`
}

// StartResponse represents a response to start request (Node.js compatible format)
type StartResponse struct {
	Response StartResponseData `json:"response"`
}

// NodeHealthCheckResponseData represents the response data for health check (Node.js format)
type NodeHealthCheckResponseData struct {
	IsAlive                  bool    `json:"isAlive"`
	XrayInternalStatusCached bool    `json:"xrayInternalStatusCached"`
	XrayVersion              *string `json:"xrayVersion"`
	NodeVersion              string  `json:"nodeVersion"`
}

// NodeHealthCheckResponse represents a response to health check request
type NodeHealthCheckResponse struct {
	Response NodeHealthCheckResponseData `json:"response"`
}

// nodeVersion is the current node version
var nodeVersion = "1.0.0"

// SetNodeVersion sets the node version (called during initialization)
func SetNodeVersion(version string) {
	nodeVersion = version
}

// Start starts the Xray process with the given configuration
func (s *XrayService) Start(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	startTime := time.Now()

	// Helper to create error response
	errorResponse := func(errMsg string) *StartResponse {
		return &StartResponse{
			Response: StartResponseData{
				IsStarted:         false,
				Version:           nil,
				Error:             &errMsg,
				SystemInformation: nil,
				NodeInformation:   NodeInformation{Version: nodeVersion},
			},
		}
	}

	// Helper to create success response
	successResponse := func(version string) *StartResponse {
		return &StartResponse{
			Response: StartResponseData{
				IsStarted:         true,
				Version:           &version,
				Error:             nil,
				SystemInformation: s.getSystemInformation(),
				NodeInformation:   NodeInformation{Version: nodeVersion},
			},
		}
	}

	// Check for concurrent processing
	if !s.isStartProcessing.CompareAndSwap(false, true) {
		errMsg := "Request already in progress"
		return errorResponse(errMsg), nil
	}
	defer s.isStartProcessing.Store(false)

	s.mu.Lock()
	defer s.mu.Unlock()

	// If Xray is online, hashed set check is enabled, and not force restart, check if restart is needed
	// This matches Node.js: if (this.isXrayOnline && !this.disableHashedSetCheck && !body.internals.forceRestart)
	if s.isXrayOnline && !s.disableHashedSetCheck && !req.Internals.ForceRestart && req.Internals.Hashes != nil && s.internal != nil {
		// First verify Xray is actually healthy
		if s.checkXrayHealth(ctx) {
			// Check if config changed
			needRestart := s.internal.IsNeedRestartCore(req.Internals.Hashes)
			if !needRestart {
				s.logger.Info("No changes detected, skipping restart",
					zap.Duration("checkTime", time.Since(startTime)))
				version := s.cachedVersion
				return &StartResponse{
					Response: StartResponseData{
						IsStarted:         true,
						Version:           &version,
						Error:             nil,
						SystemInformation: s.getSystemInformation(),
						NodeInformation:   NodeInformation{Version: nodeVersion},
					},
				}, nil
			}
		} else {
			// Health check failed, need to restart
			s.isXrayOnline = false
			s.logger.Warn("Xray Core health check failed, restarting...")
		}
	}

	// Force restart requested
	if req.Internals.ForceRestart {
		s.logger.Warn("Force restart requested")
	}

	// Check if restart is needed (hash comparison) - for first start
	if !req.Internals.ForceRestart && !s.isXrayOnline && req.Internals.Hashes != nil && s.internal != nil {
		needRestart := s.internal.IsNeedRestartCore(req.Internals.Hashes)
		if !needRestart {
			s.logger.Info("No changes detected, skipping restart",
				zap.Duration("checkTime", time.Since(startTime)))
			version := s.cachedVersion
			return &StartResponse{
				Response: StartResponseData{
					IsStarted:         true,
					Version:           &version,
					Error:             nil,
					SystemInformation: s.getSystemInformation(),
					NodeInformation:   NodeInformation{Version: nodeVersion},
				},
			}, nil
		}
	}

	// Generate full config with API, Stats, and Policy (matches Node.js generateApiConfig)
	fullConfig := generateApiConfig(req.XrayConfig)

	// Convert fullConfig to JSON bytes
	configBytes, err := json.Marshal(fullConfig)
	if err != nil {
		return errorResponse(fmt.Sprintf("failed to marshal config: %v", err)), nil
	}

	// Write config to file
	configPath := filepath.Join(s.configDir, "config.json")
	if err := os.MkdirAll(s.configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	s.logger.Info("Written Xray config", zap.String("path", configPath))

	// Extract users from config for tracking (pass hashes to store them)
	if s.internal != nil {
		if err := s.internal.ExtractUsersFromConfig(configBytes, req.Internals.Hashes); err != nil {
			s.logger.Warn("Failed to extract users from config", zap.Error(err))
		}
	}

	// Start the process via supervisord
	if err := s.supervisor.StartProcess(ctx, s.processName, true); err != nil {
		s.isXrayOnline = false
		s.logger.Error("Failed to start Xray",
			zap.Error(err),
			zap.Duration("elapsed", time.Since(startTime)))
		return errorResponse(err.Error()), nil
	}

	// Wait for gRPC to be ready and verify health
	if err := s.xtls.WaitForReady(ctx, 10*time.Second); err != nil {
		s.logger.Warn("Xray started but gRPC not ready", zap.Error(err))
	}

	// Verify Xray is actually responding
	isStarted := s.checkXrayHealth(ctx)
	if !isStarted {
		s.isXrayOnline = false
		s.logger.Error("Xray failed to start - health check failed",
			zap.Duration("elapsed", time.Since(startTime)))
		return errorResponse("Xray started but health check failed"), nil
	}

	// Get version after start
	version := s.GetVersion()

	s.isConfigured = true
	s.isXrayOnline = true
	s.logger.Info("Xray started successfully",
		zap.String("version", version),
		zap.Duration("elapsed", time.Since(startTime)))

	return successResponse(version), nil
}

// getSystemInformation returns system information for the response
func (s *XrayService) getSystemInformation() *SystemInformation {
	return &SystemInformation{
		CPUCores:    getCPUCores(),
		CPUModel:    getCPUModel(),
		MemoryTotal: getMemoryTotal(),
	}
}

// StopResponse represents a response to stop request (Node.js compatible)
type StopResponse struct {
	IsStopped bool `json:"isStopped"`
}

// Stop stops the Xray process
func (s *XrayService) Stop(ctx context.Context) (*StopResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.supervisor.StopProcess(ctx, s.processName, true); err != nil {
		return &StopResponse{IsStopped: false}, nil
	}

	s.isConfigured = false
	s.isXrayOnline = false

	// Cleanup internal state
	if s.internal != nil {
		s.internal.Cleanup()
	}

	return &StopResponse{IsStopped: true}, nil
}

// RestartRequest represents a request to restart Xray
type RestartRequest struct {
	Config       json.RawMessage `json:"config,omitempty"`
	Hashes       *InboundHashes  `json:"hashes,omitempty"`
	ForceRestart bool            `json:"forceRestart,omitempty"`
}

// RestartResponse represents a response to restart request
type RestartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Version string `json:"version,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// Restart restarts the Xray process, optionally with new config
func (s *XrayService) Restart(ctx context.Context, req *RestartRequest) (*RestartResponse, error) {
	startTime := time.Now()

	// Check for concurrent processing
	if !s.isStartProcessing.CompareAndSwap(false, true) {
		return &RestartResponse{
			Success: false,
			Message: "Request already in progress",
			Version: s.cachedVersion,
		}, nil
	}
	defer s.isStartProcessing.Store(false)

	s.mu.Lock()
	defer s.mu.Unlock()

	// If Xray is online and not force restart, check if restart is needed
	if s.isXrayOnline && !req.ForceRestart && req.Hashes != nil && s.internal != nil {
		if s.checkXrayHealth(ctx) {
			needRestart := s.internal.IsNeedRestartCore(req.Hashes)
			if !needRestart {
				s.logger.Info("No changes detected, skipping restart",
					zap.Duration("checkTime", time.Since(startTime)))
				return &RestartResponse{
					Success: true,
					Message: "No changes detected, restart skipped",
					Skipped: true,
					Version: s.cachedVersion,
				}, nil
			}
		} else {
			s.isXrayOnline = false
			s.logger.Warn("Xray Core health check failed, restarting...")
		}
	}

	if req.ForceRestart {
		s.logger.Warn("Force restart requested")
	}

	// If new config provided, write it
	if len(req.Config) > 0 {
		configPath := filepath.Join(s.configDir, "config.json")
		if err := os.WriteFile(configPath, req.Config, 0644); err != nil {
			return nil, fmt.Errorf("failed to write config file: %w", err)
		}
		s.logger.Info("Updated Xray config", zap.String("path", configPath))

		// Extract users from config for tracking (pass hashes to store them)
		if s.internal != nil {
			if err := s.internal.ExtractUsersFromConfig(req.Config, req.Hashes); err != nil {
				s.logger.Warn("Failed to extract users from config", zap.Error(err))
			}
		}
	}

	// Restart the process
	if err := s.supervisor.RestartProcess(ctx, s.processName, true); err != nil {
		s.isXrayOnline = false
		return &RestartResponse{
			Success: false,
			Message: err.Error(),
			Version: s.cachedVersion,
		}, nil
	}

	// Wait for gRPC to be ready
	if err := s.xtls.WaitForReady(ctx, 10*time.Second); err != nil {
		s.logger.Warn("Xray restarted but gRPC not ready", zap.Error(err))
	}

	// Verify health
	isStarted := s.checkXrayHealth(ctx)
	if !isStarted {
		s.isXrayOnline = false
		s.logger.Error("Xray restart failed - health check failed")
		return &RestartResponse{
			Success: false,
			Message: "Xray restarted but health check failed",
			Version: s.cachedVersion,
		}, nil
	}

	version := s.GetVersion()

	s.isConfigured = true
	s.isXrayOnline = true
	s.logger.Info("Xray restarted successfully",
		zap.String("version", version),
		zap.Duration("elapsed", time.Since(startTime)))

	return &RestartResponse{
		Success: true,
		Message: "Xray restarted successfully",
		Version: version,
	}, nil
}

// GetStatusResponse represents the status of Xray (Node.js compatible)
type GetStatusResponse struct {
	IsRunning bool    `json:"isRunning"`
	Version   *string `json:"version"`
}

// GetStatus returns the current status and version of Xray
func (s *XrayService) GetStatus(ctx context.Context) (*GetStatusResponse, error) {
	info, err := s.supervisor.GetProcessInfo(ctx, s.processName)
	if err != nil {
		return &GetStatusResponse{IsRunning: false, Version: nil}, nil
	}

	isRunning := info.StateName == supervisord.ProcessStateRunning

	var version *string
	if isRunning {
		v := s.GetVersion()
		if v != "" && v != "unknown" {
			version = &v
		}
	}

	return &GetStatusResponse{
		IsRunning: isRunning,
		Version:   version,
	}, nil
}

// IsRunning returns true if Xray is running
func (s *XrayService) IsRunning(ctx context.Context) bool {
	running, _ := s.supervisor.IsProcessRunning(ctx, s.processName)
	return running
}

// GetNodeHealthCheck returns the node health check response (Node.js compatible)
func (s *XrayService) GetNodeHealthCheck(ctx context.Context) *NodeHealthCheckResponse {
	s.mu.RLock()
	isXrayOnline := s.isXrayOnline
	s.mu.RUnlock()

	var xrayVersion *string
	if v := s.GetVersion(); v != "" && v != "unknown" {
		xrayVersion = &v
	}

	return &NodeHealthCheckResponse{
		Response: NodeHealthCheckResponseData{
			IsAlive:                  true,
			XrayInternalStatusCached: isXrayOnline,
			XrayVersion:              xrayVersion,
			NodeVersion:              nodeVersion,
		},
	}
}

// IsConfigured returns true if Xray has been configured
func (s *XrayService) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isConfigured
}

// GetConfig returns the current Xray configuration
func (s *XrayService) GetConfig() (json.RawMessage, error) {
	configPath := filepath.Join(s.configDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return data, nil
}

// GetXtlsClient returns the underlying Xray gRPC client
func (s *XrayService) GetXtlsClient() *xtls.Client {
	return s.xtls
}

// GetVersion returns the Xray version by executing "xray version" command
func (s *XrayService) GetVersion() string {
	// Return cached version if available
	if s.cachedVersion != "" {
		return s.cachedVersion
	}

	// Execute xray version command
	cmd := exec.Command(s.xrayBinary, "version")
	output, err := cmd.Output()
	if err != nil {
		s.logger.Debug("Failed to get Xray version", zap.Error(err))
		return "unknown"
	}

	// Parse version from output
	// Output format: "Xray 1.8.x (Xray, Penetrates Everything.) ..."
	version := s.parseVersionFromOutput(string(output))
	if version != "" {
		s.cachedVersion = version
	}

	return version
}

// parseVersionFromOutput extracts version string from xray version output
func (s *XrayService) parseVersionFromOutput(output string) string {
	// Use regex to match version pattern: Xray X.X.X
	re := regexp.MustCompile(`Xray\s+(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Fallback: try to find any version-like pattern
	re2 := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches2 := re2.FindStringSubmatch(output)
	if len(matches2) >= 2 {
		return matches2[1]
	}

	// Try line by line parsing
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Xray") {
			// Extract version from line like "Xray 1.8.4 (Xray, ...)"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return "unknown"
}

// ClearVersionCache clears the cached version
func (s *XrayService) ClearVersionCache() {
	s.cachedVersion = ""
}

// System information helper functions
func getCPUCores() int {
	return runtime.NumCPU()
}

func getCPUModel() string {
	// Try to read CPU model from /proc/cpuinfo on Linux
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "Unknown"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

func getMemoryTotal() string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Try to get system total memory from /proc/meminfo on Linux
	data, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1] + " kB"
				}
			}
		}
	}

	// Fallback to Go runtime stats
	return fmt.Sprintf("%d MB", memStats.Sys/1024/1024)
}
