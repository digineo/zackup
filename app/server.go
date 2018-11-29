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
	"github.com/gorilla/mux"
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
	mux.HandleFunc("/-/metrics", srv.promStub).Methods(http.MethodGet)
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

func (srv *server) promStub(w http.ResponseWriter, r *http.Request) {
	log.Error("todo")
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

var tpl = template.Must(template.New("index").Funcs(template.FuncMap{
	"fmtTime": func(t interface{}) template.HTML {
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
			return template.HTML("n/a")
		}
		const format = "2006-01-02, 15:04"
		return template.HTML(val.In(time.Local).Truncate(time.Second).Format(format))
	},

	"fmtDuration": func(d interface{}) template.HTML {
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
			return template.HTML("n/a")
		}
		return template.HTML(val.String())
	},

	"statusClass": func(m HostMetrics) string {
		switch m.Status() {
		case StatusFailed:
			return "table-warning"
		case StatusSuccess:
			return "table-success"
		case StatusRunning:
			return "table-warning"
		}
		return ""
	},

	"statusIcon": func(m HostMetrics) string {
		switch m.Status() {
		case StatusFailed:
			return "fas fa-times"
		case StatusSuccess:
			return "fas fa-check"
		case StatusRunning:
			return "far fa-clock"
		}
		return "fas fa-question"
	},
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
					active since {{ fmtTime .Scheduler.NextRun }}
				{{ else }}
					next run scheduled at {{ fmtTime .Scheduler.NextRun }}
				{{ end }}
				</span>
				Date: {{ fmtTime .Time }}
			</caption>
			<thead>
				<tr>
					<th class="text-right">Status</th>
					<th class="text-right">Host</th>
					<th class="text-right">last started</th>
					<th class="text-right">last succeeded</th>
					<th>duration</th>
					<th class="text-right">last failed</th>
					<th>duration</th>
				</tr>
			</thead>
			<tbody>
			{{ range .Hosts }}
				<tr class="{{ statusClass . }}">
					<td class="text-right">{{ .Status }} <i class="{{ statusIcon . }} fa-fw"></i></td>
					<td class="text-right">{{ .Host }}</td>
					<td class="text-right">{{ fmtTime .StartedAt }}</td>
					{{ if .SucceededAt }}
						<td class="text-right">{{ fmtTime .SucceededAt }}</td>
						<td>{{ fmtDuration .SuccessDuration }}</td>
					{{ else }}
						<td class="text-right">n/a</td>
						<td>n/a</td>
					{{ end }}
					{{ if .FailedAt }}
						<td class="text-right">{{ fmtTime .FailedAt }}</td>
						<td>{{ fmtDuration .FailureDuration }}</td>
					{{ else }}
						<td class="text-right">n/a</td>
						<td>n/a</td>
					{{ end }}
				</tr>
			{{ end }}
			</tbody>
		</table>
	</main>
</body>
</html>
`))
