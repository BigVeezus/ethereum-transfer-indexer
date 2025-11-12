package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TransfersProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eth_transfers_processed_total",
			Help: "Total number of ERC-20 transfer events processed",
		},
		[]string{"status"},
	)

	TransfersProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "eth_transfers_processing_duration_seconds",
			Help:    "Time spent processing transfer events",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	BlocksProcessedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "eth_blocks_processed_total",
			Help: "Total number of blocks processed",
		},
	)

	IngestionErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eth_ingestion_errors_total",
			Help: "Total number of ingestion errors",
		},
		[]string{"type"},
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// RPC provider metrics
	RPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of RPC requests by provider and method",
		},
		[]string{"provider", "method"},
	)

	RPCErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_errors_total",
			Help: "Total number of RPC errors by provider and error code",
		},
		[]string{"provider", "error_code"},
	)

	RPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_request_duration_seconds",
			Help:    "RPC request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
		},
		[]string{"provider", "method"},
	)

	CurrentBlockHeight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "current_block_height",
			Help: "Current block height per provider",
		},
		[]string{"provider"},
	)
)
