package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/clash-version/remnawave-node-go/pkg/crypto"
)

// Config holds all configuration values
type Config struct {
	// Server settings
	NodePort int

	// Secret key (contains TLS certs and JWT public key)
	SecretKey string

	// Parsed payload from SECRET_KEY
	NodePayload *crypto.NodePayload

	// Xray gRPC settings
	XtlsIP   string
	XtlsPort int

	// Feature flags
	DisableHashedSetCheck bool
}

// Internal API constants
const (
	XrayInternalAPIPort = 61001
	SupervisordRPCPort  = 61002
	XrayGRPCPort        = 61000
)

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}

	// NODE_PORT (required)
	portStr := getEnv("NODE_PORT", "3000")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid NODE_PORT: %w", err)
	}
	cfg.NodePort = port

	// SECRET_KEY (required)
	cfg.SecretKey = os.Getenv("SECRET_KEY")
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("SECRET_KEY is required")
	}

	// Parse SECRET_KEY payload
	payload, err := crypto.ParseNodePayload(cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SECRET_KEY: %w", err)
	}
	cfg.NodePayload = payload

	// XTLS settings
	cfg.XtlsIP = getEnv("XTLS_IP", "127.0.0.1")
	xtlsPortStr := getEnv("XTLS_PORT", "61000")
	xtlsPort, err := strconv.Atoi(xtlsPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid XTLS_PORT: %w", err)
	}
	cfg.XtlsPort = xtlsPort

	// Feature flags
	cfg.DisableHashedSetCheck = getEnvBool("DISABLE_HASHED_SET_CHECK", false)

	return cfg, nil
}

// GetXtlsAddress returns the full address for Xray gRPC connection
func (c *Config) GetXtlsAddress() string {
	return fmt.Sprintf("%s:%d", c.XtlsIP, c.XtlsPort)
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool returns environment variable as bool or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
