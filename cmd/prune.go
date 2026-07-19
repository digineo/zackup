package cmd

import (
	"github.com/digineo/zackup/app"
	"github.com/spf13/cobra"
)

// pruneCmd represents the prune command
var pruneCmd = &cobra.Command{
	Use:   "prune [host [...]]",
	Short: "Prunes backups per-host ZFS dataset",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			args = tree.Hosts()
		}

		for _, host := range args {
			job := tree.Host(host)
			if job == nil {
				log.WithField("prune", host).Warn("unknown host, ignoring")
				continue
			}

			app.PruneSnapshots(job)
		}
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
