package cmd

import (
	"git.digineo.de/digineo/zackup/app"
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints a list of hosts and their backup status (last success, size)",
	Run: func(cmd *cobra.Command, args []string) {
		for _, host := range app.ExportState() {
			pp.Println(host)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
