package app

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusExporter is a Prometheus metrics endpoint.
type PrometheusExporter struct {
}

var _ prometheus.Collector = (*PrometheusExporter)(nil) // type check

var (
	hostLabels      = []string{"host"}
	descLastSuccess = prometheus.NewDesc(joinLabels("last_success"), "duration since last success in seconds", hostLabels, nil)
)

// Describe implements the prometheus.Collector interface
func (e PrometheusExporter) Describe(c chan<- *prometheus.Desc) {
	c <- descLastSuccess
}

// Collect implements the prometheus.Collector interface
func (e PrometheusExporter) Collect(c chan<- prometheus.Metric) {
	now := time.Now()

	for _, metrics := range state.export() {
		labels := []string{metrics.Host}

		since := float64(-1)
		if succeededAt := metrics.SucceededAt; succeededAt != nil {
			since = float64(now.Sub(*succeededAt) / time.Second)
		}
		c <- prometheus.MustNewConstMetric(descLastSuccess, prometheus.CounterValue, since, labels...)
	}
}

func joinLabels(parts ...string) string {
	parts = append([]string{"zackup"}, parts...)
	return strings.Join(parts, "_")
}

func init() {
	prom := &PrometheusExporter{}
	prometheus.MustRegister(prom)
}
