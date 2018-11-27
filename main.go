package main

import (
	"os"
	"time"

	"git.digineo.de/digineo/zackup/app"
	"git.digineo.de/digineo/zackup/cmd"
	"git.digineo.de/digineo/zackup/config"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	log = logrus.WithField("prefix", "main")
)

func main() {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		logrus.SetFormatter(&prefixed.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.StampMilli,
		})
	}

	fin := cmd.Execute(func(tree config.Tree) {
		svc := tree.Service()
		if svc == nil {
			return
		}

		cmd.ResizeQueue(int(svc.Parallel))
		app.RootDataset = svc.RootDataset
		app.MountBase = svc.MountBase
	})
	fin()
}
