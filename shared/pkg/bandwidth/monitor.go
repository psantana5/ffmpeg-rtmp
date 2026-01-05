package bandwidth

import (
"fmt"
"net/http"
"sync"

"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promhttp"
)

// BandwidthMonitor tracks HTTP request/response bandwidth
type BandwidthMonitor struct {
// Metrics
bytesReceived  *prometheus.CounterVec
bytesSent      *prometheus.CounterVec
requestSize    *prometheus.HistogramVec
responseSize   *prometheus.HistogramVec
totalBandwidth *prometheus.GaugeVec

mu sync.RWMutex
}

// NewBandwidthMonitor creates a new bandwidth monitor
func NewBandwidthMonitor() *BandwidthMonitor {
bm := &BandwidthMonitor{
bytesReceived: prometheus.NewCounterVec(
prometheus.CounterOpts{
Name: "scheduler_http_request_bytes_total",
Help: "Total bytes received in HTTP requests by the scheduler",
},
[]string{"method", "endpoint"},
),
bytesSent: prometheus.NewCounterVec(
prometheus.CounterOpts{
Name: "scheduler_http_response_bytes_total",
Help: "Total bytes sent in HTTP responses by the scheduler",
},
[]string{"method", "endpoint", "status"},
),
requestSize: prometheus.NewHistogramVec(
prometheus.HistogramOpts{
Name:    "scheduler_http_request_size_bytes",
Help:    "HTTP request size in bytes received by scheduler",
Buckets: prometheus.ExponentialBuckets(100, 10, 8),
},
[]string{"method", "endpoint"},
),
responseSize: prometheus.NewHistogramVec(
prometheus.HistogramOpts{
Name:    "scheduler_http_response_size_bytes",
Help:    "HTTP response size in bytes sent by scheduler",
Buckets: prometheus.ExponentialBuckets(100, 10, 8),
},
[]string{"method", "endpoint", "status"},
),
totalBandwidth: prometheus.NewGaugeVec(
prometheus.GaugeOpts{
Name: "scheduler_bandwidth_bytes_per_second",
Help: "Scheduler bandwidth in bytes per second",
},
[]string{"direction"}, // "inbound", "outbound", "total"
),
}

prometheus.MustRegister(bm.bytesReceived)
prometheus.MustRegister(bm.bytesSent)
prometheus.MustRegister(bm.requestSize)
prometheus.MustRegister(bm.responseSize)
prometheus.MustRegister(bm.totalBandwidth)

return bm
}

// Middleware returns HTTP middleware that tracks bandwidth
func (bm *BandwidthMonitor) Middleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
endpoint := r.URL.Path
method := r.Method

// Track request size
requestSize := r.ContentLength
if requestSize < 0 {
requestSize = 0
}

if requestSize > 0 {
bm.bytesReceived.WithLabelValues(method, endpoint).Add(float64(requestSize))
bm.requestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
}

// Wrap response writer
rw := &responseWriter{
ResponseWriter: w,
statusCode:     http.StatusOK,
}

next.ServeHTTP(rw, r)

// Track response size
if rw.bytesWritten > 0 {
status := fmt.Sprintf("%d", rw.statusCode)
bm.bytesSent.WithLabelValues(method, endpoint, status).Add(float64(rw.bytesWritten))
bm.responseSize.WithLabelValues(method, endpoint, status).Observe(float64(rw.bytesWritten))
}
})
}

// Handler returns HTTP handler for Prometheus metrics
func (bm *BandwidthMonitor) Handler() http.Handler {
return promhttp.Handler()
}

type responseWriter struct {
http.ResponseWriter
bytesWritten int
statusCode   int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
n, err := rw.ResponseWriter.Write(b)
rw.bytesWritten += n
return n, err
}

func (rw *responseWriter) WriteHeader(statusCode int) {
rw.statusCode = statusCode
rw.ResponseWriter.WriteHeader(statusCode)
}
