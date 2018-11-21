package main

import (
	"os"
	"time"

	"git.digineo.de/digineo/zackup/app"
	"git.digineo.de/digineo/zackup/cmd"
	"git.digineo.de/digineo/zackup/config"
	"git.digineo.de/digineo/zackup/graylog"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	log = logrus.WithField("prefix", "main")
	gl  = graylog.NewMiddleware("zackup")

	queue app.Queue
)

func main() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		logrus.SetFormatter(&prefixed.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.StampMilli,
		})
	}

	logrus.AddHook(gl)
	defer gl.Flush()

	cmd.Execute(func(tree config.Tree) {
		svc := tree.Service()
		if svc == nil {
			return
		}

		gl.SetLevel(svc.LogLevel)
		gl.SetEndpoint(svc.Graylog)

		if queue == nil {
			queue = app.NewQueue(int(svc.Parallel))
		} else {
			queue.Resize(int(svc.Parallel))
		}
	})
}
