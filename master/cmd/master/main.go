package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/master/exporters/prometheus"
	"github.com/psantana5/ffmpeg-rtmp/pkg/api"
	"github.com/psantana5/ffmpeg-rtmp/pkg/auth"
	"github.com/psantana5/ffmpeg-rtmp/pkg/bandwidth"
	"github.com/psantana5/ffmpeg-rtmp/pkg/cleanup"
	"github.com/psantana5/ffmpeg-rtmp/pkg/logging"
	"github.com/psantana5/ffmpeg-rtmp/pkg/scheduler"
	"github.com/psantana5/ffmpeg-rtmp/pkg/shutdown"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
	tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
	"github.com/psantana5/ffmpeg-rtmp/pkg/tracing"
)

var logger *logging.Logger

func main() {
	// Command-line flags
	port := flag.String("port", "8080", "Master node port")
	dbPath := flag.String("db", "master.db", "SQLite database path (default: master.db, use empty string for in-memory)")
	dbType := flag.String("db-type", "", "Database type: 'sqlite', 'postgres' (defaults to sqlite if db path given)")
	dbDSN := flag.String("db-dsn", "", "Database connection string (for PostgreSQL: postgresql://user:pass@host/db)")
	useTLS := flag.Bool("tls", true, "Enable TLS (default: true)")
	certFile := flag.String("cert", "certs/master.crt", "TLS certificate file")
	keyFile := flag.String("key", "certs/master.key", "TLS key file")
	caFile := flag.String("ca", "", "CA certificate file for mTLS")
	requireClientCert := flag.Bool("mtls", false, "Require client certificate (mTLS)")
	generateCert := flag.Bool("generate-cert", false, "Generate self-signed certificate")
	certIPs := flag.String("cert-ips", "", "Comma-separated list of IP addresses to include in certificate SANs (e.g., '192.168.0.51,10.0.0.5')")
	certHosts := flag.String("cert-hosts", "", "Comma-separated list of hostnames to include in certificate SANs (e.g., 'depa,server1')")
	apiKeyFlag := flag.String("api-key", "", "API key for authentication (leave empty to use environment variable)")
	maxRetries := flag.Int("max-retries", 3, "Maximum job retry attempts on failure")
	enableMetrics := flag.Bool("metrics", true, "Enable Prometheus metrics endpoint")
	metricsPort := flag.String("metrics-port", "9090", "Prometheus metrics port")
	schedulerInterval := flag.Duration("scheduler-interval", 5*time.Second, "Background scheduler check interval")
	enableTracing := flag.Bool("tracing", false, "Enable distributed tracing")
	tracingEndpoint := flag.String("tracing-endpoint", "localhost:4318", "OpenTelemetry OTLP endpoint")
	enableCleanup := flag.Bool("cleanup", true, "Enable automatic cleanup of old jobs")
	cleanupRetention := flag.Int("cleanup-retention", 7, "Job retention period in days")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize file logger: /var/log/ffrtmp/master/master.log
	var err error
	logger, err = logging.NewFileLogger("master", "master", logging.ParseLevel(*logLevel), false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Get API key from flag or environment variable
	apiKey := *apiKeyFlag
	apiKeySource := ""
	if apiKey == "" {
		// Try MASTER_API_KEY first, then FFMPEG_RTMP_API_KEY for backward compat
		apiKey = os.Getenv("MASTER_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("FFMPEG_RTMP_API_KEY")
		}
		if apiKey != "" {
			apiKeySource = "environment variable"
		}
	} else {
		apiKeySource = "command-line flag"
	}

	logger.Info("Starting FFmpeg RTMP Distributed Master Node (Production Mode)")
	logger.Info(fmt.Sprintf("Port: %s", *port))
	logger.Info(fmt.Sprintf("Max Retries: %d", *maxRetries))
	logger.Info(fmt.Sprintf("Metrics Enabled: %v", *enableMetrics))
	if *enableMetrics {
		logger.Info(fmt.Sprintf("Metrics Port: %s", *metricsPort))
	}

	// Generate self-signed certificate if requested
	if *generateCert {
		logger.Info("Generating self-signed certificate...")
		if err := os.MkdirAll("certs", 0755); err != nil {
			logger.Fatal(fmt.Sprintf("Failed to create certs directory: %v", err))
		}

		// Parse IP addresses and hostnames from comma-separated strings
		var sans []string
		if *certIPs != "" {
			ips := strings.Split(*certIPs, ",")
			for _, ip := range ips {
				ip = strings.TrimSpace(ip)
				if ip != "" {
					sans = append(sans, ip)
				}
			}
		}
		if *certHosts != "" {
			hosts := strings.Split(*certHosts, ",")
			for _, host := range hosts {
				host = strings.TrimSpace(host)
				if host != "" {
					sans = append(sans, host)
				}
			}
		}

		if err := tlsutil.GenerateSelfSignedCert(*certFile, *keyFile, "master", sans...); err != nil {
			logger.Fatal(fmt.Sprintf("Failed to generate certificate: %v", err))
		}
		logger.Info("Certificate generated successfully")
		logger.Info(fmt.Sprintf("  Certificate: %s", *certFile))
		logger.Info(fmt.Sprintf("  Key: %s", *keyFile))
		if len(sans) > 0 {
			logger.Info(fmt.Sprintf("  Additional SANs: %v", sans))
		}
		return // Exit after generating certificate
	}

	// Create store based on configuration
	var dataStore store.Store

	// Check for environment variable overrides
	envDBType := os.Getenv("DATABASE_TYPE")
	envDBDSN := os.Getenv("DATABASE_DSN")
	
	// Priority: env vars > flags > defaults
	if envDBType != "" {
		*dbType = envDBType
	}
	if envDBDSN != "" {
		*dbDSN = envDBDSN
	}

	// Determine database type
	if *dbType == "" && *dbDSN != "" {
		// Infer type from DSN
		if strings.HasPrefix(*dbDSN, "postgres") {
			*dbType = "postgres"
		}
	}
	if *dbType == "" && *dbPath != "" {
		*dbType = "sqlite"
	}

	// Create appropriate store
	if *dbType == "postgres" || *dbType == "postgresql" {
		if *dbDSN == "" {
			logger.Fatal("PostgreSQL requires --db-dsn or DATABASE_DSN environment variable")
		}
		logger.Info(fmt.Sprintf("Using PostgreSQL database"))
		logger.Info(fmt.Sprintf("DSN: %s", maskPassword(*dbDSN)))
		
		pgStore, err := store.NewStore(store.Config{
			Type:            "postgres",
			DSN:             *dbDSN,
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		})
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to create PostgreSQL store: %v", err))
		}
		dataStore = pgStore
		logger.Info("✓ PostgreSQL connected successfully")
		logger.Info("✓ Persistent storage enabled with production-grade database")
	} else if *dbPath != "" {
		logger.Info(fmt.Sprintf("Using SQLite database: %s", *dbPath))
		sqliteStore, sErr := store.NewSQLiteStore(*dbPath)
		if sErr != nil {
			logger.Fatal(fmt.Sprintf("Failed to create SQLite store: %v", sErr))
		}
		dataStore = sqliteStore
		defer sqliteStore.Close()
		logger.Info("✓ Persistent storage enabled (data will survive restarts)")
	} else {
		logger.Info("WARNING: Using in-memory store (data will not persist)")
		logger.Info("Consider using --db flag with a database path for production")
		dataStore = store.NewMemoryStore()
	}

	// Ensure we can close the store on shutdown
	if closer, ok := dataStore.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// Setup authentication if API key provided
	if apiKey != "" {
		logger.Info(fmt.Sprintf("✓ API authentication enabled (source: %s)", apiKeySource))
	} else {
		logger.Info("WARNING: No API key provided - API is open!")
		logger.Info("For production, you must provide an API key:")
		logger.Info("  1. Set environment variable: export MASTER_API_KEY=your-secure-key")
		logger.Info("  2. Or use flag: --api-key=your-secure-key")
		logger.Info("To generate a secure key:")
		logger.Info("  openssl rand -base64 32")
	}

	// Create API handler with retry support
	handler := api.NewMasterHandlerWithRetry(dataStore, *maxRetries)

	// Initialize distributed tracing if enabled
	var tracerProvider *tracing.Provider
	if *enableTracing {
		logger.Info("Initializing distributed tracing...")
		var err error
		tracerProvider, err = tracing.InitTracer(tracing.Config{
			ServiceName:    "ffrtmp-master",
			ServiceVersion: "1.0.0",
			Environment:    "production",
			OTLPEndpoint:   *tracingEndpoint,
			Enabled:        true,
		})
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to initialize tracing: %v", err))
		}
		logger.Info(fmt.Sprintf("✓ Distributed tracing enabled (endpoint: %s)", *tracingEndpoint))
		
		// Ensure graceful shutdown of tracer
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tracerProvider.Shutdown(ctx); err != nil {
				logger.Info(fmt.Sprintf("Error shutting down tracer: %v", err))
			}
		}()
	}

	// Create router
	router := mux.NewRouter()

	// Add tracing middleware first (before auth) if enabled
	if *enableTracing && tracerProvider != nil {
		router.Use(tracing.HTTPMiddleware(tracerProvider, "ffrtmp-master"))
		logger.Info("✓ Tracing middleware enabled")
	}

	// Add bandwidth monitoring middleware
	bandwidthMonitor := bandwidth.NewBandwidthMonitor()
	router.Use(bandwidthMonitor.Middleware)
	logger.Info("✓ Bandwidth monitoring enabled")

	// Add authentication middleware if API key is set
	if apiKey != "" {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip auth for health endpoint
				if r.URL.Path == "/health" {
					next.ServeHTTP(w, r)
					return
				}

				// Check API key in Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
					return
				}

				// Simple bearer token check with constant-time comparison
				expectedAuth := "Bearer " + apiKey
				if !auth.SecureCompare(authHeader, expectedAuth) {
					http.Error(w, "Invalid API key", http.StatusUnauthorized)
					return
				}

				next.ServeHTTP(w, r)
			})
		})
	}

	handler.RegisterRoutes(router)

	// Add metrics endpoint if enabled
	var metricsExporter *prometheus.MasterExporter
	var metricsSrv *http.Server
	if *enableMetrics {
		logger.Info("✓ Prometheus metrics endpoint enabled")
		metricsExporter = prometheus.NewMasterExporter(dataStore, bandwidthMonitor)

		// Set metrics recorder in handler
		handler.SetMetricsRecorder(metricsExporter)

		// Create separate server for metrics
		metricsRouter := mux.NewRouter()
		metricsRouter.Handle("/metrics", metricsExporter).Methods("GET")
		metricsRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		}).Methods("GET")

		metricsSrv = &http.Server{
			Addr:         ":" + *metricsPort,
			Handler:      metricsRouter,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		// Start metrics server in background
		go func() {
			logger.Info(fmt.Sprintf("Metrics server listening on :%s", *metricsPort))
			logger.Info("  GET  /metrics (Prometheus format)")
			logger.Info("  GET  /health")
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Info(fmt.Sprintf("Metrics server error: %v", err))
			}
		}()
	}

	// Start automatic cleanup manager
	var cleanupMgr *cleanup.CleanupManager
	if *enableCleanup {
		logger.Info("Initializing cleanup manager...")
		cleanupConfig := cleanup.CleanupConfig{
			Enabled:          true,
			JobRetentionDays: *cleanupRetention,
			CleanupInterval:  24 * time.Hour,
			VacuumInterval:   7 * 24 * time.Hour,
			DeleteBatchSize:  100,
		}
		cleanupMgr = cleanup.NewCleanupManager(cleanupConfig, dataStore)
		cleanupMgr.Start()
		logger.Info(fmt.Sprintf("✓ Cleanup manager started (retention: %d days)", *cleanupRetention))
	}

	// Start background scheduler
	sched := scheduler.New(dataStore, *schedulerInterval)
	sched.Start()
	logger.Info(fmt.Sprintf("Background scheduler started (interval: %v)", *schedulerInterval))

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Setup TLS if enabled
	if *useTLS {
		logger.Info("TLS enabled")
		if *requireClientCert {
			logger.Info("mTLS enabled - requiring client certificates")
		}

		// Check if certificates exist
		if _, err := os.Stat(*certFile); os.IsNotExist(err) {
			logger.Info(fmt.Sprintf("Certificate file not found: %s", *certFile))
			logger.Info("Generating self-signed certificate...")
			if err := os.MkdirAll("certs", 0755); err != nil {
				logger.Fatal(fmt.Sprintf("Failed to create certs directory: %v", err))
			}
			if err := tlsutil.GenerateSelfSignedCert(*certFile, *keyFile, "master"); err != nil {
				logger.Fatal(fmt.Sprintf("Failed to generate certificate: %v", err))
			}
			logger.Info("✓ Self-signed certificate generated")
		}

		tlsConfig, err := tlsutil.LoadTLSConfig(*certFile, *keyFile, *caFile, *requireClientCert)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to load TLS config: %v", err))
		}
		srv.TLSConfig = tlsConfig
	} else {
		logger.Info("WARNING: TLS disabled")
		logger.Info("This is NOT recommended for production!")
		logger.Info("Enable with --tls flag or set --tls=false explicitly to suppress this warning")
	}

	// Initialize graceful shutdown manager
	shutdownMgr := shutdown.New(30 * time.Second)
	
	// Register shutdown handlers (LIFO order)
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Closing database connection...")
		if closer, ok := dataStore.(interface{ Close() error }); ok {
			return closer.Close()
		}
		return nil
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		if cleanupMgr != nil {
			logger.Info("Stopping cleanup manager...")
			cleanupMgr.Stop()
		}
		return nil
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Stopping scheduler...")
		sched.Stop()
		return nil
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Stopping metrics server...")
		if metricsSrv != nil {
			return metricsSrv.Shutdown(ctx)
		}
		return nil
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Stopping HTTP server...")
		return srv.Shutdown(ctx)
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Closing logger...")
		return logger.Close()
	})

	// Start server in goroutine
	go func() {
		logger.Info(fmt.Sprintf("Master node listening on :%s", *port))
		logger.Info("API endpoints:")
		logger.Info("  POST   /nodes/register")
		logger.Info("  GET    /nodes")
		logger.Info("  POST   /nodes/{id}/heartbeat")
		logger.Info("  POST   /jobs")
		logger.Info("  GET    /jobs")
		logger.Info("  GET    /jobs/next?node_id=<id>")
		logger.Info("  POST   /results")
		logger.Info("  GET    /health")

		var err error
		if *useTLS {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Fatal(fmt.Sprintf("Failed to start server: %v", err))
		}
	}()

	// Wait for shutdown signal
	shutdownMgr.Wait()
	logger.Info("Shutdown signal received")

	// Execute shutdown handlers
	shutdownMgr.Shutdown()

	logger.Info("Master server shutdown complete")
}

// maskPassword masks the password in a database DSN for logging
func maskPassword(dsn string) string {
// postgresql://user:password@host/db -> postgresql://user:****@host/db
if idx := strings.Index(dsn, "://"); idx != -1 {
afterScheme := dsn[idx+3:]
if atIdx := strings.Index(afterScheme, "@"); atIdx != -1 {
beforeAt := afterScheme[:atIdx]
if colonIdx := strings.Index(beforeAt, ":"); colonIdx != -1 {
user := beforeAt[:colonIdx]
rest := dsn[idx+3+atIdx:]
return dsn[:idx+3] + user + ":****" + rest
}
}
}
return dsn
}
