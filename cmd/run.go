package cmd

import "github.com/spf13/cobra"

var runParallel = 0

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run [host [...]]",
	Short: "Creates backups and stores them in a local per-host ZFS dataset",
	Run: func(cmd *cobra.Command, args []string) {
		if runParallel > 0 {
			queue.Resize(runParallel)
		}

		if len(args) == 0 {
			args = tree.Hosts()
		}

		for _, host := range args {
			job := tree.Host(host)
			if job == nil {
				log.WithField("job", host).Warn("unknown host, ignoring")
				continue
			}
			queue.Enqueue(job)
		}
		queue.Wait()
	},
}

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(runCmd)
	runCmd.PersistentFlags().IntVarP(&runParallel, "parallel", "P", 0,
		"Run at most `N` jobs parallel (overrides service config value, if > 0)")
}
