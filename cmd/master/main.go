package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/api"
	"github.com/psantana5/ffmpeg-rtmp/pkg/auth"
	"github.com/psantana5/ffmpeg-rtmp/pkg/metrics"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
	tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
)

func main() {
	// Command-line flags
	port := flag.String("port", "8080", "Master node port")
	dbPath := flag.String("db", "master.db", "SQLite database path (default: master.db, use empty string for in-memory)")
	useTLS := flag.Bool("tls", true, "Enable TLS (default: true)")
	certFile := flag.String("cert", "certs/master.crt", "TLS certificate file")
	keyFile := flag.String("key", "certs/master.key", "TLS key file")
	caFile := flag.String("ca", "", "CA certificate file for mTLS")
	requireClientCert := flag.Bool("mtls", false, "Require client certificate (mTLS)")
	generateCert := flag.Bool("generate-cert", false, "Generate self-signed certificate")
	certIPs := flag.String("cert-ips", "", "Comma-separated list of IP addresses to include in certificate SANs (e.g., '192.168.0.51,10.0.0.5')")
	certHosts := flag.String("cert-hosts", "", "Comma-separated list of hostnames to include in certificate SANs (e.g., 'depa,server1')")
	apiKeyFlag := flag.String("api-key", "", "API key for authentication (leave empty to disable, or use FFMPEG_RTMP_API_KEY env var)")
	apiKey := flag.String("api-key", os.Getenv("MASTER_API_KEY"), "API key for authentication (default: from MASTER_API_KEY env var)")
	maxRetries := flag.Int("max-retries", 3, "Maximum job retry attempts on failure")
	enableMetrics := flag.Bool("metrics", true, "Enable Prometheus metrics endpoint")
	metricsPort := flag.String("metrics-port", "9090", "Prometheus metrics port")
	flag.Parse()

	// Get API key from flag or environment variable
	apiKey := *apiKeyFlag
	apiKeySource := ""
	if apiKey == "" {
		apiKey = os.Getenv("FFMPEG_RTMP_API_KEY")
		if apiKey != "" {
			apiKeySource = "environment variable"
		}
	} else {
		apiKeySource = "command-line flag"
	}

	log.Println("Starting FFmpeg RTMP Distributed Master Node (Production Mode)")
	log.Printf("Port: %s", *port)
	log.Printf("Max Retries: %d", *maxRetries)
	log.Printf("Metrics Enabled: %v", *enableMetrics)
	if *enableMetrics {
		log.Printf("Metrics Port: %s", *metricsPort)
	}

	// Generate self-signed certificate if requested
	if *generateCert {
		log.Println("Generating self-signed certificate...")
		if err := os.MkdirAll("certs", 0755); err != nil {
			log.Fatalf("Failed to create certs directory: %v", err)
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
			log.Fatalf("Failed to generate certificate: %v", err)
		}
		log.Println("Certificate generated successfully")
		log.Printf("  Certificate: %s", *certFile)
		log.Printf("  Key: %s", *keyFile)
		if len(sans) > 0 {
			log.Printf("  Additional SANs: %v", sans)
		}
		return // Exit after generating certificate
	}

	// Create store
	var dataStore store.Store
	
	if *dbPath != "" {
		log.Printf("Using SQLite database: %s", *dbPath)
		sqliteStore, sErr := store.NewSQLiteStore(*dbPath)
		if sErr != nil {
			log.Fatalf("Failed to create SQLite store: %v", sErr)
		}
		dataStore = sqliteStore
		defer sqliteStore.Close()
		log.Println("✓ Persistent storage enabled (data will survive restarts)")
	} else {
		log.Println("WARNING: Using in-memory store (data will not persist)")
		log.Println("Consider using --db flag with a database path for production")
		dataStore = store.NewMemoryStore()
	}

	// Setup authentication if API key provided
	if apiKey != "" {
		log.Printf("API authentication enabled (source: %s)", apiKeySource)
	if *apiKey != "" {
		log.Println("✓ API authentication enabled")
	} else {
		log.Println("ERROR: No API key provided")
		log.Println("For production, you must provide an API key:")
		log.Println("  1. Set environment variable: export MASTER_API_KEY=your-secure-key")
		log.Println("  2. Or use flag: --api-key=your-secure-key")
		log.Println("To generate a secure key:")
		log.Println("  openssl rand -base64 32")
		log.Fatalf("API key required for production deployment")
	}

	// Create API handler with retry support
	handler := api.NewMasterHandlerWithRetry(dataStore, *maxRetries)

	// Create router
	router := mux.NewRouter()
	
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
	if *enableMetrics {
		log.Println("✓ Metrics endpoint enabled")
		metricsCollector := metrics.NewCollector(dataStore)
		
		// Create separate server for metrics
		metricsRouter := mux.NewRouter()
		metricsRouter.Handle("/metrics", metricsCollector).Methods("GET")
		metricsRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		}).Methods("GET")
		
		metricsSrv := &http.Server{
			Addr:         ":" + *metricsPort,
			Handler:      metricsRouter,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		
		// Start metrics server in background
		go func() {
			log.Printf("Metrics server listening on :%s", *metricsPort)
			log.Println("  GET  /metrics (Prometheus format)")
			log.Println("  GET  /health")
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}

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
		log.Println("✓ TLS enabled")
		if *requireClientCert {
			log.Println("✓ mTLS enabled - requiring client certificates")
		}

		// Check if certificates exist
		if _, err := os.Stat(*certFile); os.IsNotExist(err) {
			log.Printf("Certificate file not found: %s", *certFile)
			log.Println("Generating self-signed certificate...")
			if err := os.MkdirAll("certs", 0755); err != nil {
				log.Fatalf("Failed to create certs directory: %v", err)
			}
			if err := tlsutil.GenerateSelfSignedCert(*certFile, *keyFile, "master"); err != nil {
				log.Fatalf("Failed to generate certificate: %v", err)
			}
			log.Println("✓ Self-signed certificate generated")
		}

		tlsConfig, err := tlsutil.LoadTLSConfig(*certFile, *keyFile, *caFile, *requireClientCert)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		srv.TLSConfig = tlsConfig
	} else {
		log.Println("WARNING: TLS disabled")
		log.Println("This is NOT recommended for production!")
		log.Println("Enable with --tls flag or set --tls=false explicitly to suppress this warning")
	}

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Master node listening on :%s", *port)
		log.Println("API endpoints:")
		log.Println("  POST   /nodes/register")
		log.Println("  GET    /nodes")
		log.Println("  POST   /nodes/{id}/heartbeat")
		log.Println("  POST   /jobs")
		log.Println("  GET    /jobs")
		log.Println("  GET    /jobs/next?node_id=<id>")
		log.Println("  POST   /results")
		log.Println("  GET    /health")

		var err error
		if *useTLS {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop

	log.Println("Shutting down gracefully...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
