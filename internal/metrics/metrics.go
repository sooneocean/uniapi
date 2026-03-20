package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uniapi_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "uniapi_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60},
	}, []string{"method", "path"})

	ProviderRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uniapi_provider_requests_total",
		Help: "Total provider API requests",
	}, []string{"provider", "model", "status"})

	ProviderLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "uniapi_provider_latency_seconds",
		Help:    "Provider API latency",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"provider", "model"})

	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uniapi_active_connections",
		Help: "Current active connections",
	})

	TokensProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uniapi_tokens_total",
		Help: "Total tokens processed",
	}, []string{"direction", "model"}) // direction: "input" or "output"
)
