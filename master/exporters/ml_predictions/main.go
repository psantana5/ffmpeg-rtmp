package main

/*
#cgo LDFLAGS: -lffmpeg_ml
#include <stdlib.h>

// C structures matching Rust FFI
typedef struct {
    float bitrate_kbps;
    unsigned int resolution_width;
    unsigned int resolution_height;
    float frame_rate;
    float frame_drop;
    float motion_intensity;
} PredictionFeatures;

typedef struct {
    float predicted_vmaf;
    float predicted_psnr;
    float predicted_cost_usd;
    float predicted_co2_kg;
    float confidence;
    unsigned int recommended_bitrate_kbps;
} PredictionResult;

// Forward declarations of Rust FFI functions
extern void* ml_load_model(const char* path);
extern int ml_predict(const void* model, const PredictionFeatures* features, PredictionResult* result);
extern int ml_save_model(const void* model, const char* path);
extern void ml_model_free(void* model);
*/
import "C"

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

const (
	defaultPort = 9505
)

var (
	model            unsafe.Pointer
	modelMutex       sync.RWMutex
	modelPath        string
	modelLoadTime    time.Time
	modelLoadTimeMux sync.RWMutex
)

// loadModel loads the ML model from disk
func loadModel(path string) error {
	modelMutex.Lock()
	defer modelMutex.Unlock()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	newModel := C.ml_load_model(cPath)
	if newModel == nil {
		return fmt.Errorf("failed to load model from %s", path)
	}

	// Free old model if exists
	if model != nil {
		C.ml_model_free(model)
	}

	model = newModel
	
	// Update model load time
	modelLoadTimeMux.Lock()
	modelLoadTime = time.Now()
	modelLoadTimeMux.Unlock()
	
	log.Printf("Loaded ML model from %s", path)
	return nil
}

// predict makes a prediction using the loaded model
func predict(features C.PredictionFeatures) (C.PredictionResult, error) {
	modelMutex.RLock()
	defer modelMutex.RUnlock()

	if model == nil {
		return C.PredictionResult{}, fmt.Errorf("model not loaded")
	}

	var result C.PredictionResult
	ret := C.ml_predict(model, &features, &result)
	if ret != 0 {
		return C.PredictionResult{}, fmt.Errorf("prediction failed")
	}

	return result, nil
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Exporter metadata
	fmt.Fprintln(w, "# HELP ml_exporter_up ML exporter is running")
	fmt.Fprintln(w, "# TYPE ml_exporter_up gauge")
	
	modelMutex.RLock()
	modelLoaded := model != nil
	modelMutex.RUnlock()
	
	if modelLoaded {
		fmt.Fprintln(w, "ml_exporter_up 1")
	} else {
		fmt.Fprintln(w, "ml_exporter_up 0")
	}

	if !modelLoaded {
		return
	}

	// Generate predictions for common encoding configurations
	configurations := []struct {
		name             string
		bitrateKbps      float32
		width            uint32
		height           uint32
		fps              float32
		frameDrop        float32
		motionIntensity  float32
	}{
		{"720p30_2mbps", 2000, 1280, 720, 30, 0.01, 0.5},
		{"1080p30_4mbps", 4000, 1920, 1080, 30, 0.01, 0.5},
		{"1080p60_6mbps", 6000, 1920, 1080, 60, 0.01, 0.6},
		{"4k30_15mbps", 15000, 3840, 2160, 30, 0.005, 0.7},
	}

	// VMAF predictions
	fmt.Fprintln(w, "# HELP qoe_predicted_vmaf Predicted VMAF quality score (0-100)")
	fmt.Fprintln(w, "# TYPE qoe_predicted_vmaf gauge")

	// PSNR predictions
	fmt.Fprintln(w, "# HELP qoe_predicted_psnr Predicted PSNR quality score (dB)")
	fmt.Fprintln(w, "# TYPE qoe_predicted_psnr gauge")

	// Cost predictions
	fmt.Fprintln(w, "# HELP cost_predicted_usd Predicted transcoding cost (USD)")
	fmt.Fprintln(w, "# TYPE cost_predicted_usd gauge")

	// CO2 predictions
	fmt.Fprintln(w, "# HELP cost_predicted_co2_kg Predicted CO2 emissions (kg)")
	fmt.Fprintln(w, "# TYPE cost_predicted_co2_kg gauge")

	// Prediction confidence
	fmt.Fprintln(w, "# HELP prediction_confidence Model prediction confidence (0-1)")
	fmt.Fprintln(w, "# TYPE prediction_confidence gauge")

	// Recommended bitrate
	fmt.Fprintln(w, "# HELP recommended_bitrate_kbps Recommended optimal bitrate (kbps)")
	fmt.Fprintln(w, "# TYPE recommended_bitrate_kbps gauge")

	for _, config := range configurations {
		features := C.PredictionFeatures{
			bitrate_kbps:        C.float(config.bitrateKbps),
			resolution_width:    C.uint(config.width),
			resolution_height:   C.uint(config.height),
			frame_rate:          C.float(config.fps),
			frame_drop:          C.float(config.frameDrop),
			motion_intensity:    C.float(config.motionIntensity),
		}

		result, err := predict(features)
		if err != nil {
			log.Printf("Prediction failed for %s: %v", config.name, err)
			continue
		}

		labels := fmt.Sprintf("config=\"%s\",bitrate=\"%.0fkbps\",resolution=\"%dx%d\",fps=\"%.0f\"",
			config.name, config.bitrateKbps, config.width, config.height, config.fps)

		fmt.Fprintf(w, "qoe_predicted_vmaf{%s} %.2f\n", labels, float64(result.predicted_vmaf))
		fmt.Fprintf(w, "qoe_predicted_psnr{%s} %.2f\n", labels, float64(result.predicted_psnr))
		fmt.Fprintf(w, "cost_predicted_usd{%s} %.6f\n", labels, float64(result.predicted_cost_usd))
		fmt.Fprintf(w, "cost_predicted_co2_kg{%s} %.6f\n", labels, float64(result.predicted_co2_kg))
		fmt.Fprintf(w, "prediction_confidence{%s} %.4f\n", labels, float64(result.confidence))
		fmt.Fprintf(w, "recommended_bitrate_kbps{%s} %d\n", labels, uint32(result.recommended_bitrate_kbps))
	}

	// Model metadata
	fmt.Fprintln(w, "# HELP ml_model_version Current ML model version")
	fmt.Fprintln(w, "# TYPE ml_model_version gauge")
	fmt.Fprintln(w, "ml_model_version{version=\"1.0.0\"} 1")

	fmt.Fprintln(w, "# HELP ml_model_last_update_timestamp Last model update time (Unix timestamp)")
	fmt.Fprintln(w, "# TYPE ml_model_last_update_timestamp gauge")
	modelLoadTimeMux.RLock()
	lastUpdate := modelLoadTime.Unix()
	modelLoadTimeMux.RUnlock()
	fmt.Fprintf(w, "ml_model_last_update_timestamp %d\n", lastUpdate)

	// Exporter info
	fmt.Fprintln(w, "# HELP ml_exporter_info ML exporter information")
	fmt.Fprintln(w, "# TYPE ml_exporter_info gauge")
	fmt.Fprintln(w, "ml_exporter_info{version=\"1.0.0\",language=\"go\",ml_backend=\"rust\"} 1")
}

// healthHandler handles the /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	modelMutex.RLock()
	modelLoaded := model != nil
	modelMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if modelLoaded {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","model_loaded":true,"timestamp":%d}`, time.Now().Unix())
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"error","model_loaded":false,"timestamp":%d}`, time.Now().Unix())
	}
}

// reloadHandler handles the /reload endpoint
func reloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := loadModel(modelPath)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"status":"error","message":"%s"}`, err.Error())
		log.Printf("Failed to reload model: %v", err)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok","message":"Model reloaded successfully"}`)
		log.Println("Model reloaded successfully")
	}
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	modelPathFlag := flag.String("model-path", "/app/ml_models/model.json", "Path to ML model file")
	flag.Parse()

	// Override with environment variables if set
	if envPort := os.Getenv("ML_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	if envModelPath := os.Getenv("MODEL_PATH"); envModelPath != "" {
		*modelPathFlag = envModelPath
	}

	modelPath = *modelPathFlag

	log.Println("Starting ML Predictions Exporter (Go + Rust)")
	log.Printf("Model path: %s", modelPath)

	// Load initial model
	if err := loadModel(modelPath); err != nil {
		log.Printf("Warning: Failed to load ML model from %s: %v. Exporter will serve empty metrics until model is loaded.", modelPath, err)
	}

	// Ensure model is freed on exit
	defer func() {
		if model != nil {
			C.ml_model_free(model)
		}
	}()

	// Register HTTP handlers
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/reload", reloadHandler)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Metrics endpoint: http://0.0.0.0:%d/metrics", *port)
	log.Printf("Health endpoint: http://0.0.0.0:%d/health", *port)
	log.Printf("Reload endpoint: http://0.0.0.0:%d/reload", *port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
