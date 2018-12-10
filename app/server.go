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
	logger *logrus.Entry // unified logging

	*http.Server
}

// NewHTTP sets a new web server up, mainly for metrics, but also for
// a quick overview. Use
func NewHTTP(bind string, port uint16) HTTP {
	srv := &server{
		logger: log.WithField("prefix", "http"),

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
		return "table-danger"
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
		return "fas fa-spinner fa-pulse"
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
	<script>
		function toggleTimes() {
			document.querySelectorAll("tbody time").forEach(el => {
				if (!el.dataset.text) {
					el.dataset.text = el.innerText
				}
				if (el.innerText === el.dataset.text) {
					el.innerText = el.getAttribute("title")
				} else {
					el.innerText = el.dataset.text
				}
			})
		}

		document.addEventListener("DOMContentLoaded", () => {
			const btn = document.querySelector("button.js-toggle-times")
			if (btn) {
				btn.addEventListener("click", ev => {
					ev.preventDefault()
					toggleTimes()
				})
			}
		})
	</script>
</head>

<body>
	<main class="container-fluid">
		<h1>zackup overview</h1>
		<table class="table table-sm table-hover table-striped">
			<caption class="small">
				<button class="btn btn-sm btn-outline-secondary js-toggle-times float-right" type="button">
					Toggle times
				</button>
				Date: {{ fmtTime .Time false }}
			</caption>
			<thead>
				<tr>
					<th rowspan="2">Host</th>
					<th rowspan="2">Status</th>
					<th rowspan="2">last started</th>
					<th rowspan="2" colspan="2" class="text-center">last succeeded</th>
					<th rowspan="2" colspan="2" class="text-center">last failed</th>
					<th rowspan="2">scheduled for</th>
					<th colspan="2">Space used</th>
					<th rowspan="2" class="text-right">Compression factor</th>
				</tr>
				<tr>
					<th class="text-right">Total</th>
					<th class="text-right">by Snapshots</th>
				</tr>
			</thead>
			<tbody>
			{{ range .Hosts }}
				<tr>
					<td><tt>{{ .Host }}</tt></td>
					<td class="{{ statusClass . }}">
						<i class="{{ statusIcon . }} fa-fw"></i>&nbsp;{{ .Status }}
					</td>
					{{ if .StartedAt.IsZero }}
						<td>{{ na }}</td>
						<td colspan="2">{{ na }}</td>
						<td colspan="2">{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
						<td>{{ na }}</td>
					{{ else }}
						<td>{{ fmtTime .StartedAt true }}</td>
						{{ if .SucceededAt }}
							<td class="text-right">{{ fmtTime .SucceededAt true }}</td>
							<td><span class="badge badge-secondary">{{ fmtDuration .SuccessDuration }}</span></td>
						{{ else }}
							<td colspan="2">{{ na }}</td>
						{{ end }}
						{{ if .FailedAt }}
							<td class="text-right">{{ fmtTime .FailedAt true }}</td>
							<td><span class="badge badge-secondary">{{ fmtDuration .FailureDuration }}</span></td>
						{{ else }}
							<td colspan="2">{{ na }}</td>
						{{ end }}
						<td>
							{{ if .ScheduledAt }}
								{{ fmtTime .ScheduledAt true }}
							{{ else }}
								{{ na }}
							{{ end }}
						</td>
						<td class="text-right">{{ humanBytes .SpaceUsedTotal }}</td>
						<td class="text-right">{{ humanBytes .SpaceUsedBySnapshots }}</td>
						<td class="text-right">{{ printf "%0.2f" .CompressionFactor }}</td>
					{{ end }}
				</tr>
			{{ end }}
			</tbody>
		</table>
	</main>
</body>
</html>
`))
