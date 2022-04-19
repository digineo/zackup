package app

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/digineo/zackup/graylog"
	humanize "github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Embed the file content as string.
//go:embed static
var staticFiles embed.FS

// HTTP allows interaction with the zackup webserver.
type HTTP interface {
	// Start will start the HTTP server.
	Start()

	// Stop will graceful shut down the HTTP server.
	Stop()
}

type server struct {
	logger *logrus.Entry // unified logging

	*http.Server
}

// NewHTTP sets a new web server up, mainly for metrics, but also for
// a quick overview.
func NewHTTP(listen string) HTTP {
	srv := &server{
		logger: log.WithField("prefix", "http"),

		Server: &http.Server{
			Addr:         listen,
			ReadTimeout:  90 * time.Second,
			WriteTimeout: 1 * time.Second,
		},
	}

	mux := mux.NewRouter()
	mux.Handle("/-/metrics", promhttp.Handler()).Methods(http.MethodGet)
	mux.HandleFunc("/", srv.handleIndex).Methods(http.MethodGet)
	mux.Use(graylog.NewMuxLogger(srv.logger))

	srv.Server.Handler = mux
	return srv
}

func (srv *server) Start() {
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		srv.logger.WithError(err).Error("unexpected shutdown")
	}
}

func (srv *server) Stop() {
	if err := srv.Shutdown(context.Background()); err != nil {
		srv.logger.WithError(err).Error("unexpected shutdown")
	}
}

func (srv *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Hosts []HostMetrics
		Time  time.Time
	}{
		Hosts: state.export(),
		Time:  time.Now().UTC(),
	}
	var buf bytes.Buffer

	if err := tpl.Execute(&buf, data); err != nil {
		srv.logger.WithError(err).Error("failed to execute index template")
		http.Error(w, "error building status table", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
}

func tplFmtTime(t interface{}, human bool) template.HTML {
	var val *time.Time
	switch ref := t.(type) {
	case time.Time:
		val = &ref
	case *time.Time:
		val = ref
	default:
		return template.HTML("invalid date")
	}

	if t == nil || val == nil {
		return tplUnavailable()
	}

	*val = val.In(time.Local).Truncate(time.Second)
	var timeTag string
	if human {
		timeTag = fmt.Sprintf(`<time datetime="%s" title="%s">%s</time>`,
			val.Format(time.RFC3339),
			val.Format("2006-01-02, 15:04"),
			humanize.Time(*val))
	} else {
		timeTag = fmt.Sprintf(`<time datetime="%s">%s</time>`,
			val.Format(time.RFC3339),
			val.Format("2006-01-02, 15:04"))
	}
	return template.HTML(timeTag)
}

func tplFmtDuration(d interface{}) template.HTML {
	var val *time.Duration
	switch ref := d.(type) {
	case time.Duration:
		val = &ref
	case *time.Duration:
		val = ref
	default:
		return template.HTML("invalid duration")
	}

	if d == nil || val == nil {
		return tplUnavailable()
	}
	if *val > 10*time.Second {
		*val = val.Truncate(100 * time.Millisecond)
	}
	return template.HTML(val.String())
}

func tplStatusClass(m HostMetrics) string {
	switch m.Status() {
	case StatusFailed:
		return "table-danger"
	case StatusSuccess:
		return "table-success"
	case StatusRunning:
		return "table-warning"
	case StatusPrimed, StatusUnknown:
		fallthrough
	default:
		return ""
	}
}

func tplStatusIcon(m HostMetrics) string {
	switch m.Status() {
	case StatusFailed:
		return "fas fa-times"
	case StatusSuccess:
		return "fas fa-check"
	case StatusRunning:
		return "fas fa-spinner fa-pulse"
	case StatusPrimed:
		return "far fa-clock"
	case StatusUnknown:
		fallthrough
	default:
		return "fas fa-question"
	}
}

func tplUnavailable() template.HTML {
	return template.HTML(`<abbr class="text-muted" title="not available">n/a</abbr>`)
}

func tplHumanBytes(val uint64) template.HTML {
	return template.HTML(humanize.Bytes(val))
}

func tplUsageDetails(m HostMetrics) string {
	var buf bytes.Buffer
	buf.WriteString("dataset: ")
	buf.WriteString(humanize.Bytes(m.SpaceUsedByDataset))
	buf.WriteString(", snapshots: ")
	buf.WriteString(humanize.Bytes(m.SpaceUsedBySnapshots))
	buf.WriteString(", children: ")
	buf.WriteString(humanize.Bytes(m.SpaceUsedByChildren))
	buf.WriteString(", reserved: ")
	buf.WriteString(humanize.Bytes(m.SpaceUsedByRefReservation))
	return buf.String()
}

func tplPercentUsage(m HostMetrics, val uint64) float64 {
	r := 100 * float64(val) / float64(m.SpaceUsedTotal())
	return math.Floor(r*100) / 100
}

var tpl = template.Must(template.New("index.html").Funcs(template.FuncMap{
	"fmtTime":      tplFmtTime,
	"fmtDuration":  tplFmtDuration,
	"statusClass":  tplStatusClass,
	"statusIcon":   tplStatusIcon,
	"na":           tplUnavailable,
	"humanBytes":   tplHumanBytes,
	"usageDetails": tplUsageDetails,
	"percentUsage": tplPercentUsage,
}).ParseFS(staticFiles, "static/index.html"))
