package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/digineo/zackup/app"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	serveBind        = "127.0.0.1"
	servePort uint16 = 3000
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts zackup as deamon.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		log.WithFields(logrus.Fields{
			"bind": serveBind,
			"port": int(servePort),
		}).Info("Start HTTP server")

		sched := app.NewScheduler(queue)
		go sched.Start()

		srv := app.NewHTTP(serveBind, servePort)
		go srv.Start()

		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.WithField("signal", (<-ch).String()).Warn("Stopping HTTP server")
		sched.Stop()
		srv.Stop()
		log.Info("Shutdown.")
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVarP(&serveBind, "bind", "b", serveBind, "`address` to bind to")
	serveCmd.PersistentFlags().Uint16VarP(&servePort, "port", "p", servePort, "`port` to bind to")
}
