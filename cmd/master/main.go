package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/api"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

func main() {
	port := flag.String("port", "8080", "Master node port")
	flag.Parse()

	log.Println("Starting FFmpeg RTMP Distributed Master Node")
	log.Printf("Port: %s", *port)

	// Create store
	store := store.NewMemoryStore()

	// Create API handler
	handler := api.NewMasterHandler(store)

	// Create router
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
