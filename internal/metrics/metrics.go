package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	EmailsEnqueued = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "senda_emails_enqueued_total",
		Help: "Total emails accepted and enqueued for sending",
	})

	EmailsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_emails_sent_total",
			Help: "Total emails sent by status",
		},
		[]string{"status", "adapter", "tenant", "workspace"},
	)

	EmailsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "senda_emails_failed_total",
		Help: "Total emails that failed permanently",
	})

	EmailSendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "senda_email_send_duration_seconds",
			Help:    "Time to process email send",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"adapter"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "senda_http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// TODO(queue-depth): River does not expose a lightweight queue-depth API without
	// a direct DB query. Wiring a periodic sampler requires a dedicated goroutine and
	// DB access in the metrics layer, which is disproportionate for a single gauge.
	// Implement once a River queue-stats helper is available or a metrics collector
	// pattern is established for background jobs.
	QueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "senda_queue_depth",
			Help: "Number of pending jobs in queue",
		},
		[]string{"queue"},
	)

	ProviderErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_provider_errors_total",
			Help: "Errors from email providers",
		},
		[]string{"adapter", "error_type"},
	)

	NegativeSignals = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_email_negative_signals_total",
			Help: "Total bounce and complaint events by type",
		},
		[]string{"tenant", "workspace", "signal_type"},
	)
)

// Register registers all Senda metrics with the default Prometheus registry.
func Register() {
	prometheus.MustRegister(
		EmailsEnqueued, EmailsSent, EmailsFailed, EmailSendDuration,
		HTTPRequestDuration, HTTPRequestsTotal,
		QueueDepth, ProviderErrors, NegativeSignals,
	)
}
