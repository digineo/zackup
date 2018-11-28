package cmd

import (
	"git.digineo.de/digineo/zackup/app"
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

		app.StartHTTP(serveBind, servePort)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVarP(&serveBind, "bind", "b", serveBind, "address to bind to")
	serveCmd.PersistentFlags().Uint16VarP(&servePort, "port", "p", servePort, "port to bind to")
}
