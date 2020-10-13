package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/digineo/zackup/app"
	humanize "github.com/dustin/go-humanize"
	"github.com/k0kubun/pp"
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

// statusCmd represents the status command. Its Run func deals mostly
// with output formatting.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints a list of hosts and their backup status",
	Run: func(cmd *cobra.Command, args []string) {
		if verbosity > 2 {
			pp.Println(tree)
			return
		}

		exported := app.ExportState()

		wantAll := len(args) == 0
		wantOnly := make(map[string]bool)
		for _, host := range args {
			wantOnly[host] = true
		}

		longest := 0
		for _, host := range exported {
			if !wantAll && !wantOnly[host.Host] {
				continue
			}
			if l := len(host.Host); l > longest {
				longest = l
			}
		}
		ws := strings.Repeat(" ", longest)

		for _, host := range exported {
			if !wantAll && !wantOnly[host.Host] {
				continue
			}

			s := host.Status()
			fmt.Printf("%-[1]*s  status            %s\n", longest, host.Host, colorize(s))

			// we don't know anything yet if primed
			if s != app.StatusPrimed {
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

				fmt.Printf("%s  space used        %s (%s snapshots, %s dataset, %s children, %s refreservation)\n", ws,
					humanize.Bytes(host.SpaceUsedTotal()),
					humanize.Bytes(host.SpaceUsedBySnapshots),
					humanize.Bytes(host.SpaceUsedByDataset),
					humanize.Bytes(host.SpaceUsedByChildren),
					humanize.Bytes(host.SpaceUsedByRefReservation))
				fmt.Printf("%s  compression       %0.2fx\n", ws, host.CompressionFactor)
			}

			if verbosity > 0 {
				job := tree.Host(host.Host)
				pp.Println(job)
			}
		}
	},
}

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(statusCmd)
}
