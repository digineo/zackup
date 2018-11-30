package cmd

import (
	"fmt"
	"strings"
	"time"

	"git.digineo.de/digineo/zackup/app"
	humanize "github.com/dustin/go-humanize"
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
	return dur.Truncate(time.Millisecond).String()
}

func colorize(s app.MetricStatus) string {
	var color string

	switch s {
	case app.StatusUnknown:
		color = "1;30" // "brigth black"
	case app.StatusPrimed:
		color = "0;36" // cyan
	case app.StatusSuccess:
		color = "1;32" // green
	case app.StatusFailed:
		color = "1;31" // red
	case app.StatusRunning:
		color = "0;34" // blue
	}
	if color != "" {
		return fmt.Sprintf("\033[%sm%s\033[0m", color, s.String())
	}
	return s.String()
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
			fmt.Printf("%-[1]*s  status            %s\n", longest, host.Host, colorize(s))

			if s == app.StatusPrimed {
				// we don't know anything yet
				continue
			}

			if s == app.StatusUnknown || s == app.StatusRunning {
				fmt.Printf("%s  started           %s\n", ws, statusTime(&host.StartedAt))
			}
			if s == app.StatusUnknown || s == app.StatusSuccess {
				t := statusTime(host.SucceededAt)
				d := statusDur(host.SuccessDuration)
				fmt.Printf("%s  succeeded at      %s (took %s)\n", ws, t, d)
			}
			if s == app.StatusUnknown || s == app.StatusFailed {
				t := statusTime(host.FailedAt)
				d := statusDur(host.FailureDuration)
				fmt.Printf("%s  failed at         %s (took %s)\n", ws, t, d)
			}

			fmt.Printf("%s  space used        %s\n", ws, humanize.Bytes(host.SpaceUsedTotal))
			fmt.Printf("%s  used by snapshots %s\n", ws, humanize.Bytes(host.SpaceUsedBySnapshots))
			fmt.Printf("%s  compression       %0.2fx\n", ws, host.CompressionFactor)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
