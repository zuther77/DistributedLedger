package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)


var (
	// Counter: +1 on each successfull order accepted
	OrderProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name : "order_processed_total",
		Help : "Orders successfully accepted by the API",
	})

	// Histogram: observe duration in seconds each match handle
	MatchingLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "order_matching_latency_seconds", 
		Help: "Time to handle one order in matching engine",
		Buckets: prometheus.DefBuckets,
	})

	SettlementErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "database_transaction_errors_total",
		Help: "Settlement transactions that failed",
	})
)