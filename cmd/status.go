package cmd

import (
	"fmt"
	"strings"
	"time"

	"git.digineo.de/digineo/zackup/app"
	"github.com/spf13/cobra"
)

func statusTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Truncate(time.Second).Format(time.RFC3339)
}

func statusDur(dur time.Duration) string {
	if dur <= 0 {
		return "-"
	}
	return dur.Truncate(time.Second).String()
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints a list of hosts and their backup status",
	Run: func(cmd *cobra.Command, args []string) {
		exported := app.ExportState()

		longest := 0
		for _, host := range exported {
			if l := len(host.Host); l > longest {
				longest = l
			}
		}
		ws := strings.Repeat(" ", longest)

		for _, host := range exported {
			s := host.Status()
			fmt.Printf("%-[1]*s  status       %s\n", longest, host.Host, s)

			if s == app.StatusUnknown || s == app.StatusRunning {
				fmt.Printf("%s  started      %s\n", ws, statusTime(&host.StartedAt))
			}
			if s == app.StatusUnknown || s == app.StatusSuccess {
				t := statusTime(host.SucceededAt)
				d := statusDur(host.SuccessDuration)
				fmt.Printf("%s  succeeded at %s (took %s)\n", ws, t, d)
			}
			if s == app.StatusUnknown || s == app.StatusFailed {
				t := statusTime(host.FailedAt)
				d := statusDur(host.FailureDuration)
				fmt.Printf("%s  failed at    %s (took %s)\n", ws, t, d)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
