package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/clash-version/remnawave-node-go/internal/config"
	"github.com/clash-version/remnawave-node-go/internal/middleware"
	"github.com/clash-version/remnawave-node-go/internal/services"
	"github.com/clash-version/remnawave-node-go/pkg/logger"
	"github.com/clash-version/remnawave-node-go/pkg/supervisord"
	"github.com/clash-version/remnawave-node-go/pkg/xtls"
)

// Server represents the HTTP server
type Server struct {
	cfg            *config.Config
	log            *logger.Logger
	mainServer     *http.Server
	internalServer *http.Server
	router         *gin.Engine
	internalRouter *gin.Engine

	// Services
	xrayService     *services.XrayService
	handlerService  *services.HandlerService
	statsService    *services.StatsService
	visionService   *services.VisionService
	internalService *services.InternalService

	// Clients
	xtlsClient       *xtls.Client
	supervisorClient *supervisord.Client
}

// New creates a new server instance
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create main router
	router := gin.New()
	router.Use(middleware.Recovery(log))
	router.Use(middleware.Logger(log))

	// Create internal router (no auth required)
	internalRouter := gin.New()
	internalRouter.Use(middleware.Recovery(log))

	// Create Xray gRPC client
	xtlsClient, err := xtls.NewClient(&xtls.Config{
		IP:   cfg.XtlsIP,
		Port: cfg.XtlsPort,
	}, log.Desugar())
	if err != nil {
		return nil, fmt.Errorf("failed to create Xray client: %w", err)
	}

	// Create Supervisord client
	supervisorURL := fmt.Sprintf("http://127.0.0.1:%d/RPC2", config.SupervisordRPCPort)
	supervisorClient := supervisord.NewClient(&supervisord.Config{
		URL: supervisorURL,
	}, log.Desugar())

	// Create services
	// Internal service must be created first as other services depend on it
	internalService := services.NewInternalService(&services.InternalConfig{
		DisableHashCheck: cfg.DisableHashedSetCheck,
	}, log.Desugar())

	xrayService := services.NewXrayService(&services.XrayConfig{
		ProcessName:           "xray",
		ConfigDir:             "/var/lib/remnawave-node",
		XrayBinary:            "xray",
		DisableHashedSetCheck: cfg.DisableHashedSetCheck,
	}, supervisorClient, xtlsClient, internalService, log.Desugar())

	handlerService := services.NewHandlerService(xtlsClient, internalService, log.Desugar())
	statsService := services.NewStatsService(xtlsClient, log.Desugar())
	visionService := services.NewVisionService(&services.VisionConfig{
		BlockTag: "block",
	}, xtlsClient, log.Desugar())

	srv := &Server{
		cfg:              cfg,
		log:              log,
		router:           router,
		internalRouter:   internalRouter,
		xtlsClient:       xtlsClient,
		supervisorClient: supervisorClient,
		xrayService:      xrayService,
		handlerService:   handlerService,
		statsService:     statsService,
		visionService:    visionService,
		internalService:  internalService,
	}

	// Setup routes
	srv.setupRoutes()

	return srv, nil
}

// Start starts both main and internal servers
func (s *Server) Start() error {
	errChan := make(chan error, 2)

	// Start main server with mTLS
	go func() {
		if err := s.startMainServer(); err != nil {
			errChan <- fmt.Errorf("main server error: %w", err)
		}
	}()

	// Start internal server (HTTP only, localhost)
	go func() {
		if err := s.startInternalServer(); err != nil {
			errChan <- fmt.Errorf("internal server error: %w", err)
		}
	}()

	// Wait for any error
	return <-errChan
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

// startInternalServer starts the internal HTTP server on localhost
func (s *Server) startInternalServer() error {
	addr := fmt.Sprintf("127.0.0.1:%d", config.XrayInternalAPIPort)
	s.internalServer = &http.Server{
		Addr:              addr,
		Handler:           s.internalRouter,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	s.log.Infow("Starting internal server",
		"addr", addr,
	)

	return s.internalServer.ListenAndServe()
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

// Shutdown gracefully shuts down the servers
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Shutdown main server
	if s.mainServer != nil {
		if err := s.mainServer.Shutdown(shutdownCtx); err != nil {
			s.log.Errorw("Main server shutdown error", "error", err)
		}
	}

	// Shutdown internal server
	if s.internalServer != nil {
		if err := s.internalServer.Shutdown(shutdownCtx); err != nil {
			s.log.Errorw("Internal server shutdown error", "error", err)
		}
	}

	return nil
}
