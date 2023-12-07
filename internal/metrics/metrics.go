package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	PromRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "requests",
		Help: "Number of requests",
	})

	PromSuccesses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "successes",
		Help: "Number of successful requests",
	})

	PromLatencyMSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_ms",
		Help: "RPC latency distributions in milliseconds.",
		// 50 exponential buckets ranging from 0.5 ms to 3 minutes
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(0.5, 1.3, 50),
	})

	PromLatencyUSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_us",
		Help: "RPC latency distributions in microseconds.",
		// 50 exponential buckets ranging from 1 us to 2.4 seconds
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(1, 1.35, 50),
	})
	PromLatencyNSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_ns",
		Help: "RPC latency distributions in nanoseconds.",
		// 50 exponential buckets ranging from 1 ns to 0.4 seconds
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(1, 1.5, 50),
	})
)

func RegisterMetrics() {
	prometheus.MustRegister(PromRequests)
	prometheus.MustRegister(PromSuccesses)
	prometheus.MustRegister(PromLatencyMSHistogram)
	prometheus.MustRegister(PromLatencyUSHistogram)
	prometheus.MustRegister(PromLatencyNSHistogram)
}
