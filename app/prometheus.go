package app

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusExporter is a Prometheus metrics endpoint.
type PrometheusExporter struct{}

func init() {
	prom := &PrometheusExporter{}
	prometheus.MustRegister(prom)
}

var (
	hostLabels       = []string{"host"}
	descLastSuccess  = prometheus.NewDesc("zackup_last_success", "duration since last success in seconds", hostLabels, nil)
	descLastDuration = prometheus.NewDesc("zackup_last_duration", "duration of last successful run in seconds", hostLabels, nil)
	descSpaceTotal   = prometheus.NewDesc("zackup_space_used", "total space used for backups in bytes", hostLabels, nil)
	descSpaceSnaps   = prometheus.NewDesc("zackup_space_used_by_snapshots", "space used by snapshots in bytes", hostLabels, nil)
	descCompression  = prometheus.NewDesc("zackup_compression", "compression ratio", hostLabels, nil)
)

// Describe implements the prometheus.Collector interface
func (e PrometheusExporter) Describe(c chan<- *prometheus.Desc) {
	c <- descLastSuccess
	c <- descLastDuration
	c <- descSpaceTotal
	c <- descSpaceSnaps
	c <- descCompression
}

// Collect implements the prometheus.Collector interface
func (e PrometheusExporter) Collect(c chan<- prometheus.Metric) {
	now := time.Now()

	for _, m := range state.export() {
		labels := []string{m.Host}

		since, dur := float64(-1), float64(-1)
		if m.SucceededAt != nil {
			since = float64(now.Sub(*m.SucceededAt) / time.Second)
			dur = float64(m.SuccessDuration) / float64(time.Second)
		}

		c <- prometheus.MustNewConstMetric(descLastSuccess, prometheus.CounterValue, since, labels...)
		c <- prometheus.MustNewConstMetric(descLastDuration, prometheus.GaugeValue, dur, labels...)
		c <- prometheus.MustNewConstMetric(descSpaceTotal, prometheus.GaugeValue, float64(m.SpaceUsedTotal), labels...)
		c <- prometheus.MustNewConstMetric(descSpaceSnaps, prometheus.GaugeValue, float64(m.SpaceUsedBySnapshots), labels...)
		c <- prometheus.MustNewConstMetric(descCompression, prometheus.GaugeValue, m.CompressionFactor, labels...)
	}
}
