package cmd

import (
	"os"

	"git.digineo.de/digineo/zackup/config"
	"git.digineo.de/digineo/zackup/graylog"
	"github.com/digineo/goldflags"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	log = logrus.WithField("prefix", "commands")

	tree         = config.NewTree("")
	treeRoot     = config.DefaultRoot
	treeCallback func(config.Tree)

	gl         = graylog.NewMiddleware("zackup")
	glEndpoint string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "zackup",
	Short:   "A small utility to backup remote hosts into local ZFS datasets.",
	Version: goldflags.VersionString(),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// The (optional) callback function is called once the config tree was (re-) loaded.
func Execute(callback func(config.Tree)) func() {
	if callback != nil && treeCallback == nil {
		treeCallback = callback
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	return func() {
		if glEndpoint != "" {
			gl.Flush()
		}
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&treeRoot, "root", "r", treeRoot, "config root directory")
	rootCmd.PersistentFlags().StringVarP(&glEndpoint, "gelf", "l", glEndpoint, "GELF endpoint (Graylog remote logging)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if glEndpoint != "" {
		logrus.AddHook(gl)
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

	if treeCallback != nil {
		treeCallback(tree)
	}

	hosts := tree.Hosts()
	injectHostArgs(hosts, runCmd)
	injectHostArgs(hosts, statusCmd)
}

func injectHostArgs(hosts []string, cmd *cobra.Command) {
	cmd.ValidArgs = hosts
	cmd.Args = cobra.OnlyValidArgs
}
