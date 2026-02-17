package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	EmailsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "senda_emails_sent_total",
			Help: "Total emails sent by status",
		},
		[]string{"status", "adapter", "tenant", "workspace"},
	)

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

	BounceRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "senda_bounce_rate",
			Help: "Bounce rate in last 24h window",
		},
		[]string{"tenant", "workspace", "bounce_type"},
	)
)

// Register registers all Senda metrics with the default Prometheus registry.
func Register() {
	prometheus.MustRegister(
		EmailsSent, EmailSendDuration,
		HTTPRequestDuration, HTTPRequestsTotal,
		QueueDepth, ProviderErrors, BounceRate,
	)
}
