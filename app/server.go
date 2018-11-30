package app

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"git.digineo.de/digineo/zackup/graylog"
	humanize "github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// HTTP allows interaction with the zackup webserver.
type HTTP interface {
	// Start will start the HTTP server.
	Start()

	// Stop will gracefull shut down the HTTP server.
	Stop()
}

type server struct {
	scheduler Scheduler     // we only need its State()
	logger    *logrus.Entry // unified logging

	*http.Server
}

// NewHTTP sets a new web server up, mainly for metrics, but also for
// a quick overview. Use
func NewHTTP(bind string, port uint16, s Scheduler) HTTP {
	srv := &server{
		scheduler: s,
		logger:    log.WithField("prefix", "http"),

		Server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", bind, port),
			ReadTimeout:  3 * time.Second,
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
	if err != nil && err != http.ErrServerClosed {
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
		Hosts     []HostMetrics
		Time      time.Time
		Scheduler SchedulerState
	}{
		Hosts:     state.export(),
		Time:      time.Now().UTC(),
		Scheduler: srv.scheduler.State(),
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
	switch t.(type) {
	case time.Time:
		ref := t.(time.Time)
		val = &ref
	case *time.Time:
		val = t.(*time.Time)
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
	switch d.(type) {
	case time.Duration:
		ref := d.(time.Duration)
		val = &ref
	case *time.Duration:
		val = d.(*time.Duration)
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
		return "table-warning"
	case StatusSuccess:
		return "table-success"
	case StatusRunning:
		return "table-warning"
	}
	return ""
}

func tplStatusIcon(m HostMetrics) string {
	switch m.Status() {
	case StatusFailed:
		return "fas fa-times"
	case StatusSuccess:
		return "fas fa-check"
	case StatusRunning:
		return "far fa-spinner fa-pulse"
	case StatusPrimed:
		return "far fa-clock"
	}
	return "fas fa-question"
}

func tplUnavailable() template.HTML {
	return template.HTML(`<abbr class="text-muted" title="not available">n/a</abbr>`)
}

func tplHumanBytes(val uint64) template.HTML {
	return template.HTML(humanize.Bytes(val))
}

var tpl = template.Must(template.New("index").Funcs(template.FuncMap{
	"fmtTime":     tplFmtTime,
	"fmtDuration": tplFmtDuration,
	"statusClass": tplStatusClass,
	"statusIcon":  tplStatusIcon,
	"na":          tplUnavailable,
	"humanBytes":  tplHumanBytes,
}).Parse(`<!doctype html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
	<title>zackup overview</title>
	<link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/css/bootstrap.min.css"
		integrity="sha384-MCw98/SFnGE8fJT3GXwEOngsV7Zt27NXFoaoApmYm81iuXoPkFOJwJ8ERdknLPMO" crossorigin="anonymous">
	<link rel="stylesheet" href="https://use.fontawesome.com/releases/v5.5.0/css/all.css"
		integrity="sha384-B4dIYHKNBt8Bc12p+WXckhzcICo0wtJAoU8YZTY5qE0Id1GSseTk6S+L3BlXeVIU" crossorigin="anonymous">
</head>

<body>
	<main class="container">
		<h1>zackup overview</h1>
		<table class="table table-sm">
			<caption class="small">
				<span class="float-right">
				{{ if .Scheduler.Active }}
					<strong class="text-success">active</strong> since {{ fmtTime .Scheduler.NextRun false }}
				{{ else }}
					next run scheduled at {{ fmtTime .Scheduler.NextRun false }}
				{{ end }}
				</span>
				Date: {{ fmtTime .Time false }}
			</caption>
			<thead>
				<tr>
					<th>Host</th>
					<th>Status</th>
					<th>last started</th>
					<th>last succeeded</th>
					<th>last failed</th>
					<th>Space used (total)</th>
					<th>Space used (by snapshots)</th>
					<th>Compression factor</th>
				</tr>
			</thead>
			<tbody>
			{{ range .Hosts }}
				<tr class="{{ statusClass . }}">
					<td><tt>{{ .Host }}</tt></td>
					<td><i class="{{ statusIcon . }} fa-fw"></i>&nbsp;{{ .Status }}</td>
					{{ if .StartedAt.IsZero }}
						<td>{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
					{{ else }}
						<td>{{ fmtTime .StartedAt true }}</td>
						<td>
							{{ if .SucceededAt }}
								{{ fmtTime .SucceededAt true }}
								<span class="badge badge-secondary">{{ fmtDuration .SuccessDuration }}</span>
							{{ else }}
								{{ na }}
							{{ end }}
						</td>
						<td>
							{{ if .FailedAt }}
								{{ fmtTime .FailedAt true }}
								<span class="badge badge-secondary">{{ fmtDuration .FailureDuration }}</span>
							{{ else }}
								{{ na }}
							{{ end }}
						</td>
						<td>{{ humanBytes .SpaceUsedTotal }}</td>
						<td>{{ humanBytes .SpaceUsedBySnapshots }}</td>
						<td>{{ printf "%0.2f" .CompressionFactor }}</td>
					{{ end }}
				</tr>
			{{ end }}
			</tbody>
		</table>
	</main>
</body>
</html>
`))
