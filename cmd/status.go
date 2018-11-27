package cmd

import (
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [host [...]]",
	Short: "Prints a list of hosts and their backup status (last success, size)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.WithField("hosts", tree.Hosts()).
				Info("dumping list of hosts")
			return
		}
		for _, host := range args {
			job := tree.Host(host)
			pp.Println(job)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
