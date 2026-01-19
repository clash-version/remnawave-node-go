package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"github.com/clash-version/remnawave-node-go/internal/config"
	"github.com/clash-version/remnawave-node-go/internal/middleware"
	"github.com/clash-version/remnawave-node-go/internal/services"
	"github.com/clash-version/remnawave-node-go/pkg/logger"
	"github.com/clash-version/remnawave-node-go/pkg/xraycore"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	log        *logger.Logger
	mainServer *http.Server
	router     *gin.Engine

	// Services
	xrayService     *services.XrayService
	handlerService  *services.HandlerService
	statsService    *services.StatsService
	visionService   *services.VisionService
	internalService *services.InternalService

	// Embedded Xray-core
	xrayCore *xraycore.Instance
}

// New creates a new server instance
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create main router
	router := gin.New()
	router.Use(middleware.Recovery(log))
	router.Use(middleware.Logger(log))

	// Create embedded Xray-core instance
	xrayCoreInstance := xraycore.New(&xraycore.Config{
		Logger: log.Desugar(),
	})

	// Create services
	// Internal service must be created first as other services depend on it
	internalService := services.NewInternalService(&services.InternalConfig{
		DisableHashCheck: cfg.DisableHashedSetCheck,
	}, log.Desugar())

	xrayService := services.NewXrayService(&services.XrayConfig{
		ConfigDir:             "/var/lib/remnawave-node",
		DisableHashedSetCheck: cfg.DisableHashedSetCheck,
	}, xrayCoreInstance, internalService, log.Desugar())

	handlerService := services.NewHandlerService(xrayCoreInstance, internalService, log.Desugar())
	statsService := services.NewStatsService(xrayCoreInstance, log.Desugar())
	visionService := services.NewVisionService(&services.VisionConfig{
		BlockTag: "block",
	}, xrayCoreInstance, log.Desugar())

	srv := &Server{
		cfg:             cfg,
		log:             log,
		router:          router,
		xrayCore:        xrayCoreInstance,
		xrayService:     xrayService,
		handlerService:  handlerService,
		statsService:    statsService,
		visionService:   visionService,
		internalService: internalService,
	}

	// Setup routes
	srv.setupRoutes()

	return srv, nil
}

// Start starts the main HTTP server with mTLS
func (s *Server) Start() error {
	return s.startMainServer()
}

// startMainServer starts the main HTTPS server with mTLS
func (s *Server) startMainServer() error {
	// Create TLS config
	tlsConfig, err := s.createTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	addr := fmt.Sprintf(":%d", s.cfg.NodePort)
	s.mainServer = &http.Server{
		Addr:              addr,
		Handler:           s.router,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    65536, // 64KB
	}

	s.log.Infow("Starting main server",
		"port", s.cfg.NodePort,
		"tls", true,
		"mtls", true,
	)

	// Start with TLS
	return s.mainServer.ListenAndServeTLS("", "")
}

// createTLSConfig creates the mTLS configuration
func (s *Server) createTLSConfig() (*tls.Config, error) {
	payload := s.cfg.NodePayload

	// Parse server certificate
	cert, err := tls.X509KeyPair([]byte(payload.NodeCertPem), []byte(payload.NodeKeyPem))
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Create CA cert pool for client verification
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(payload.CACertPem)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	return tlsConfig, nil
}

// Shutdown gracefully shuts down the server and Xray-core
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Stop embedded Xray-core
	if s.xrayCore != nil {
		if err := s.xrayCore.Stop(); err != nil {
			s.log.Errorw("Xray-core shutdown error", "error", err)
		}
	}

	// Shutdown main server
	if s.mainServer != nil {
		if err := s.mainServer.Shutdown(shutdownCtx); err != nil {
			s.log.Errorw("Main server shutdown error", "error", err)
		}
	}

	return nil
}
