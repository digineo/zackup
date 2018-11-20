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
			args = tree.Hosts()
		}

		for _, host := range args {
			job := tree.Host(host)
			pp.Println(host, job)
		}
	},
	// Args, ValidArgs setup in initConfig
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
