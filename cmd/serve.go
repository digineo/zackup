package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/digineo/zackup/app"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listenAddress = "127.0.0.1:3000"

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts zackup as deamon.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		log.WithFields(logrus.Fields{
			"listen": listenAddress,
		}).Info("Start HTTP server")

		sched := app.NewScheduler(queue)
		go sched.Start()

		srv := app.NewHTTP(listenAddress)
		go srv.Start()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.WithField("signal", (<-ch).String()).Warn("Stopping HTTP server")
		sched.Stop()
		srv.Stop()
		log.Info("Shutdown.")
	},
}

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVarP(&listenAddress, "listen", "l", listenAddress, "`address` to listen on")
}
