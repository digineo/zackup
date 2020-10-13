package cmd

import (
	"os"

	"github.com/digineo/goldflags"
	"github.com/digineo/zackup/app"
	"github.com/digineo/zackup/config"
	"github.com/digineo/zackup/graylog"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	log       = logrus.WithField("prefix", "commands")
	verbosity int

	tree     = config.NewTree("")
	treeRoot = config.DefaultRoot
	queue    = app.NewQueue()

	gl         = graylog.NewMiddleware("zackup")
	glEndpoint string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "zackup",
	Short:   "A small utility to backup remote hosts into local ZFS datasets.",
	Version: goldflags.VersionString(),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		gl.Flush()
		os.Exit(1)
	}
	gl.Flush()
}

func init() { //nolint:gochecknoinits
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&treeRoot, "root", "r", treeRoot, "config root `directory`")
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "increase log level (specify once for debug, twice for trace level messages)")
	rootCmd.PersistentFlags().StringVarP(&glEndpoint, "gelf", "", glEndpoint, "GELF UDP endpoint in `host:port` notation")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if glEndpoint != "" {
		gl.SetEndpoint(glEndpoint)
	}

	if verbosity == 2 {
		gl.SetLevel("trace")
		log.Debug("increase log level (trace)")
	} else if verbosity == 1 {
		gl.SetLevel("debug")
		log.Debug("increase log level (debug)")
	}

	if treeRoot == "" {
		if envRoot := os.Getenv("ZACKUP_ROOT"); envRoot != "" {
			treeRoot = envRoot
		}
	}

	l := log.WithField("root", treeRoot)
	if err := tree.SetRoot(treeRoot); err != nil {
		log.WithError(err).Fatalf("failed to read config tree")
	}
	l.Info("config tree read")

	hosts := tree.Hosts()
	injectHostArgs(hosts, runCmd)
	injectHostArgs(hosts, statusCmd)

	if svc := tree.Service(); svc != nil {
		if verbosity == 0 {
			gl.SetLevel(svc.LogLevel)
		}

		queue.Resize(int(svc.Parallel))

		if err := app.InitializeState(tree); err != nil {
			l.WithError(err).Fatalf("state initialization failed")
		}
	}
}

func injectHostArgs(hosts []string, cmd *cobra.Command) {
	cmd.ValidArgs = hosts
	cmd.Args = cobra.OnlyValidArgs
}
