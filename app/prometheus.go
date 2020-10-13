package app

import (
	"time"

	"github.com/digineo/goldflags"
	"github.com/prometheus/client_golang/prometheus"
)

// promExport models an exported field.
type promExport struct {
	name, help string
	typ        prometheus.ValueType
	value      func(m *HostMetrics) float64

	// desc is dynamically build upon first use in Desc().
	desc *prometheus.Desc
}

// promExporter exports Prometheus metrics.
type promExporter []*promExport

func init() { //nolint:gochecknoinits,funlen
	prom := &promExporter{
		&promExport{
			name: "last_success",
			help: "timestamp of last success",
			typ:  prometheus.CounterValue,
			value: func(m *HostMetrics) float64 {
				since := float64(-1)
				if m.SucceededAt != nil {
					since = float64(m.SucceededAt.Unix())
				}
				return since
			},
		},
		&promExport{
			name: "last_duration",
			help: "duration of last successful run in seconds",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				dur := float64(-1)
				if m.SucceededAt != nil {
					dur = float64(m.SuccessDuration / time.Second)
				}
				return dur
			},
		},
		&promExport{
			name: "space_used",
			help: "total space used for backups in bytes",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return float64(m.SpaceUsedTotal())
			},
		},
		&promExport{
			name: "space_used_by_snapshots",
			help: "space used by snapshots in bytes",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return float64(m.SpaceUsedBySnapshots)
			},
		},
		&promExport{
			name: "space_used_by_dataset",
			help: "space used by dataset in bytes",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return float64(m.SpaceUsedByDataset)
			},
		},
		&promExport{
			name: "space_used_by_children",
			help: "space used by children in bytes",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return float64(m.SpaceUsedByChildren)
			},
		},
		&promExport{
			name: "space_used_by_reserved",
			help: "space reserved in bytes",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return float64(m.SpaceUsedByRefReservation)
			},
		},
		&promExport{
			name: "compression",
			help: "compression ratio",
			typ:  prometheus.GaugeValue,
			value: func(m *HostMetrics) float64 {
				return m.CompressionFactor
			},
		},
	}
	prometheus.MustRegister(prom)
}

var hostLabels = []string{"host"}

var version = prometheus.NewDesc(
	"zackup_version",
	"version information",
	nil,
	map[string]string{
		"version": goldflags.VersionString(),
	},
)

func (f *promExport) Desc() *prometheus.Desc {
	if f.desc == nil {
		name := prometheus.BuildFQName("zackup", "", f.name)
		f.desc = prometheus.NewDesc(name, f.help, hostLabels, nil)
	}
	return f.desc
}

// Describe implements the prometheus.Collector interface.
func (e promExporter) Describe(c chan<- *prometheus.Desc) {
	for _, f := range e {
		c <- f.Desc()
	}
	c <- version
}

// Collect implements the prometheus.Collector interface.
func (e promExporter) Collect(c chan<- prometheus.Metric) {
	metrics := state.export()
	for i := range state.export() {
		m := metrics[i]
		for _, f := range e {
			val := f.value(&m)
			c <- prometheus.MustNewConstMetric(f.desc, f.typ, val, m.Host)
		}
	}
	c <- prometheus.MustNewConstMetric(version, prometheus.UntypedValue, 1)
}
