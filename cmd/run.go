package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [host [...]]",
	Short: "Creates backups and stores them in a local per-host ZFS dataset",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run called")
	},
	// Args, ValidArgs setup in initConfig
}

func init() {
	rootCmd.AddCommand(runCmd)
}
