package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	serveBind        = "127.0.0.1"
	servePort uint16 = 3000
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts zackup as deamon.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Println("serve called")
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVarP(&serveBind, "bind", "b", serveBind, "address to bind to")
	serveCmd.PersistentFlags().Uint16VarP(&servePort, "port", "p", servePort, "port to bind to")
}
