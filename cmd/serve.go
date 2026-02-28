package cmd

import (
	"fmt"
	"os"

	"github.com/pranavtyagi/govin/internal/api"
	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a govin API server so others can connect remotely",
	Long: `Start a lightweight HTTP server backed by the local SQLite database.

Other users set GOVIN_SERVER=http://<your-ip>:<port> and all their
CLI commands transparently hit this server — real-time sync with no setup.

Example:
  # On your machine (host):
  govin serve --port 8080

  # On friends' machines:
  export GOVIN_SERVER=http://192.168.1.10:8080
  govin balance`,
	Run: func(cmd *cobra.Command, args []string) {
		addr := fmt.Sprintf(":%d", servePort)
		if err := api.StartServer(addr); err != nil {
			fmt.Fprintln(os.Stderr, StyleError.Render("Server error: "+err.Error()))
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "Port to listen on")
	rootCmd.AddCommand(serveCmd)
}
