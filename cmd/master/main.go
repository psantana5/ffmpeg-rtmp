package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/api"
	"github.com/psantana5/ffmpeg-rtmp/pkg/auth"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
	tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
)

func main() {
	// Command-line flags
	port := flag.String("port", "8080", "Master node port")
	dbPath := flag.String("db", "", "SQLite database path (empty for in-memory)")
	useTLS := flag.Bool("tls", false, "Enable TLS")
	certFile := flag.String("cert", "certs/master.crt", "TLS certificate file")
	keyFile := flag.String("key", "certs/master.key", "TLS key file")
	caFile := flag.String("ca", "", "CA certificate file for mTLS")
	requireClientCert := flag.Bool("mtls", false, "Require client certificate (mTLS)")
	generateCert := flag.Bool("generate-cert", false, "Generate self-signed certificate")
	apiKey := flag.String("api-key", "", "API key for authentication (leave empty to disable)")
	flag.Parse()

	log.Println("Starting FFmpeg RTMP Distributed Master Node (Production Mode)")
	log.Printf("Port: %s", *port)

	// Generate self-signed certificate if requested
	if *generateCert {
		log.Println("Generating self-signed certificate...")
		if err := os.MkdirAll("certs", 0755); err != nil {
			log.Fatalf("Failed to create certs directory: %v", err)
		}
		if err := tlsutil.GenerateSelfSignedCert(*certFile, *keyFile, "master"); err != nil {
			log.Fatalf("Failed to generate certificate: %v", err)
		}
		log.Println("Certificate generated successfully")
		log.Printf("  Certificate: %s", *certFile)
		log.Printf("  Key: %s", *keyFile)
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
	} else {
		log.Println("Using in-memory store (data will not persist)")
		dataStore = store.NewMemoryStore()
	}

	// Setup authentication if API key provided
	if *apiKey != "" {
		log.Println("API authentication enabled")
		log.Printf("Using API key for authentication")
	} else {
		log.Println("WARNING: API authentication disabled (not recommended for production)")
	}

	// Create API handler
	handler := api.NewMasterHandler(dataStore)

	// Create router
	router := mux.NewRouter()
	
	// Add authentication middleware if API key is set
	if *apiKey != "" {
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
				expectedAuth := "Bearer " + *apiKey
				if !auth.SecureCompare(authHeader, expectedAuth) {
					http.Error(w, "Invalid API key", http.StatusUnauthorized)
					return
				}

				next.ServeHTTP(w, r)
			})
		})
	}

	handler.RegisterRoutes(router)

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
		log.Println("TLS enabled")
		if *requireClientCert {
			log.Println("mTLS enabled - requiring client certificates")
		}

		tlsConfig, err := tlsutil.LoadTLSConfig(*certFile, *keyFile, *caFile, *requireClientCert)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		srv.TLSConfig = tlsConfig
	} else {
		log.Println("WARNING: TLS disabled (not recommended for production)")
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
